package atomic_cmd

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	atomic_common "cli/cmd/atomic/common"
	"cli/common"

	"github.com/spf13/cobra"
)

//go:embed default/server_post_native.txt
var defaultNativeServerPost string

//go:embed default/server_get_native.txt
var defaultNativeServerGet string

// generateMain writes a main.go that wraps the user's handler function
// using the drift SDK stdin/stdout protocol.
func generateMain(dir, funcName, method string) error {
	var code string
	replacer := strings.NewReplacer("{{FUNC}}", funcName)
	switch method {
	case "post":
		code = replacer.Replace(defaultNativeServerPost)
	case "get":
		code = replacer.Replace(defaultNativeServerGet)
	}
	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0o600)
}

// TriggerSpec is the minimal trigger definition declared in source comments.
type TriggerSpec struct {
	Type     string `json:"type"`               // "queue" | "webhook" | "schedule"
	Source   string `json:"source"`             // queue name or webhook path
	Schedule string `json:"schedule,omitempty"` // Go duration string e.g. "5m" (schedule triggers)
	PollMS   int    `json:"poll_ms,omitempty"`  // polling interval ms (queue triggers)
	MaxRetry int    `json:"max_retry,omitempty"`
}

// parseTriggerComments scans all .go files in dir for // drift:trigger annotations.
//
// Supported formats:
//
//	// drift:trigger queue my-queue
//	// drift:trigger queue my-queue poll=250ms retry=5
//	// drift:trigger webhook /hooks/payment
func parseTriggerComments(dir string) ([]TriggerSpec, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var triggers []TriggerSpec
	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 — CLI reads user's source file by design
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "// drift:trigger ") {
				continue
			}
			parts := strings.Fields(strings.TrimPrefix(line, "// drift:trigger "))
			if len(parts) < 2 {
				continue
			}
			trigType := parts[0]
			if trigType != "queue" && trigType != "webhook" {
				fmt.Printf("⚠️  Unknown trigger type %q — skipping\n", trigType)
				continue
			}
			spec := TriggerSpec{
				Type:     trigType,
				Source:   parts[1],
				PollMS:   500,
				MaxRetry: 3,
			}
			for _, kv := range parts[2:] {
				if strings.HasPrefix(kv, "poll=") {
					if d, err := time.ParseDuration(strings.TrimPrefix(kv, "poll=")); err == nil {
						spec.PollMS = int(d.Milliseconds())
					}
				} else if strings.HasPrefix(kv, "retry=") {
					if n, err := strconv.Atoi(strings.TrimPrefix(kv, "retry=")); err == nil && n > 0 {
						spec.MaxRetry = n
					}
				}
			}
			triggers = append(triggers, spec)
		}
	}
	return triggers, nil
}

// parseScheduleComments scans all .go files in dir for // drift:schedule annotations.
//
// The value must be a standard 5-field cron expression (minute hour dom month dow).
//
//	// drift:schedule */5 * * * *
//	// drift:schedule 0 15 * * *
//	// drift:schedule 0 0 * * 1
func parseScheduleComments(dir string) ([]TriggerSpec, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var triggers []TriggerSpec
	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 — CLI reads user's source file by design
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "// drift:schedule ") {
				continue
			}
			expr := strings.TrimSpace(strings.TrimPrefix(line, "// drift:schedule "))
			if expr == "" {
				continue
			}
			triggers = append(triggers, TriggerSpec{
				Type:     "schedule",
				Schedule: expr,
			})
		}
	}
	return triggers, nil
}

// readDotEnvKeys parses a .env file and returns only the key names.
func readDotEnvKeys(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 — CLI tool reads user's .env file by design
	if err != nil {
		return nil, err
	}
	defer f.Close()

	re := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	var keys []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) == 2 {
			keys = append(keys, m[1])
		}
	}
	return keys, scanner.Err()
}

// readDotEnvPairs parses a .env file and returns key=value pairs as a map.
func readDotEnvPairs(path string) (map[string]string, error) {
	f, err := os.Open(path) // #nosec G304 — CLI tool reads user's .env file by design
	if err != nil {
		return nil, err
	}
	defer f.Close()

	re := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$`)
	pairs := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) == 3 {
			pairs[m[1]] = strings.Trim(m[2], "\"'")
		}
	}
	return pairs, scanner.Err()
}

// syncEnvToBackbone reads the .env file in folder, finds keys not yet in
// Backbone, prompts the user, and pushes missing ones. Returns all .env keys.
func syncEnvToBackbone(folder string) ([]string, error) {
	dotEnvPath := filepath.Join(folder, ".env")
	pairs, err := readDotEnvPairs(dotEnvPath)
	if err != nil || len(pairs) == 0 {
		return nil, nil // no .env or empty — nothing to do
	}

	// Fetch existing Backbone secret names.
	resp, err := common.DoRequest(http.MethodGet, common.APIBaseURL+"/ops/backbone/secret/list", nil)
	if err != nil {
		return nil, common.TransportError("list existing secrets", err)
	}
	defer resp.Body.Close()

	var existing []string
	if b, err := common.CheckResponse(resp, "list existing secrets"); err == nil {
		_ = json.Unmarshal(b, &existing)
	}
	existingSet := map[string]struct{}{}
	for _, k := range existing {
		existingSet[k] = struct{}{}
	}

	// Determine which keys from .env are missing from Backbone.
	var missing []string
	for k := range pairs {
		if _, ok := existingSet[k]; !ok {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		fmt.Printf("Found %d secret(s) in .env not yet in Backbone: %s\n", len(missing), strings.Join(missing, ", "))
		fmt.Print("   Push them to Backbone now? [Y/n] ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			for _, k := range missing {
				body, _ := json.Marshal(map[string]string{"name": k, "value": pairs[k]})
				r, err := common.DoJSONRequest(http.MethodPost, common.APIBaseURL+"/ops/backbone/secret/set", bytes.NewBuffer(body))
				if err != nil {
					fmt.Printf("   %s\n", common.TransportError("push secret "+k, err))
					continue
				}
				if _, err := common.CheckResponse(r, "push secret "+k); err != nil {
					fmt.Printf("   %s\n", err)
				} else {
					fmt.Printf("   %s pushed to Backbone\n", k)
				}
				r.Body.Close()
			}
		}
	}

	// Return all keys from .env (whether pre-existing or just pushed).
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	return keys, nil
}

// sendSourceToOperator uploads the compiled binary to the API as
// multipart/form-data. The whole payload is buffered (typical Go binaries
// are ~10–30 MB) so the request body can be replayed if the auto-refresh
// path needs to retry after a 401.
func sendSourceToOperator(name, method, language, auth, element, sourcePath string, envKeys []string, triggers []TriggerSpec) error {
	meta, err := json.Marshal(map[string]any{
		"name":     name,
		"method":   method,
		"language": language,
		"auth":     auth,
		"element":  element,
		"env_keys": envKeys,
		"triggers": triggers,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	f, err := os.Open(sourcePath) // #nosec G304 — CLI tool reads user's compiled binary by design
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	metaPart, err := mw.CreateFormField("metadata")
	if err != nil {
		return fmt.Errorf("failed to create metadata field: %w", err)
	}
	if _, err := metaPart.Write(meta); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	srcPart, err := mw.CreateFormFile("source", filepath.Base(sourcePath))
	if err != nil {
		return fmt.Errorf("failed to create source field: %w", err)
	}
	if _, err := io.Copy(srcPart, f); err != nil {
		return fmt.Errorf("failed to buffer source: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("failed to finalize multipart: %w", err)
	}

	resp, err := common.DoRequestWithContentType(
		http.MethodPost,
		common.APIBaseURL+"/ops/atomic",
		mw.FormDataContentType(),
		&buf,
	)
	if err != nil {
		return common.TransportError("deploy the function", err)
	}
	defer resp.Body.Close()

	if _, err := common.CheckResponse(resp, "deploy the function"); err != nil {
		return err
	}

	return nil
}

// DeployFolder builds and deploys the atomic function at folder. It is
// exported so that "drift deploy" can call it directly without going through
// the cobra command layer. When quiet is true, all per-function status
// chatter is suppressed so the manifest deploy can render its own clean
// summary line for each function.
func DeployFolder(folder, element string, quiet bool) error {
	method, name, auth, err := atomic_common.ParseAtomicMetadataFromDir(folder)
	if err != nil {
		return fmt.Errorf("failed to parse Atomic metadata: %w", err)
	}
	if !quiet {
		if element != "" {
			fmt.Printf("🚀 Deploying function '%s /%s/%s' (auth: %s, element: %s)\n", method, element, name, auth, element)
		} else {
			fmt.Printf("🚀 Deploying function '%s /%s' (auth: %s)\n", method, name, auth)
		}
	}

	envKeys, err := syncEnvToBackbone(folder)
	if err != nil {
		return fmt.Errorf("env sync failed: %w", err)
	}

	triggers, err := parseTriggerComments(folder)
	if err != nil {
		return fmt.Errorf("failed to parse trigger comments: %w", err)
	}
	schedules, err := parseScheduleComments(folder)
	if err != nil {
		return fmt.Errorf("failed to parse schedule comments: %w", err)
	}
	triggers = append(triggers, schedules...)
	if len(triggers) > 0 && !quiet {
		fmt.Printf("⚡ %d trigger(s) found: ", len(triggers))
		for i, t := range triggers {
			if i > 0 {
				fmt.Print(", ")
			}
			if t.Type == "schedule" {
				fmt.Printf("schedule(every %s)", t.Schedule)
			} else {
				fmt.Printf("%s(%s)", t.Type, t.Source)
			}
		}
		fmt.Println()
	}

	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return fmt.Errorf("failed to resolve folder path: %w", err)
	}

	namePascal := ""
	for _, seg := range strings.Split(name, "-") {
		namePascal += common.CapitalizeFirst(strings.ToLower(seg))
	}
	funcName := common.CapitalizeFirst(strings.ToLower(method)) + namePascal

	var sourcePath, language string

	language = "native"

	if err := generateMain(absFolder, funcName, method); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}
	defer os.Remove(filepath.Join(absFolder, "main.go"))

	// Back up go.mod/go.sum so `go mod tidy` (which can rewrite both to add
	// transitive requires) doesn't dirty the user's working tree.
	origGoMod, _ := os.ReadFile(filepath.Join(absFolder, "go.mod"))
	origGoSum, goSumErr := os.ReadFile(filepath.Join(absFolder, "go.sum"))
	defer func() {
		if origGoMod != nil {
			os.WriteFile(filepath.Join(absFolder, "go.mod"), origGoMod, 0o600)
		}
		if goSumErr == nil {
			os.WriteFile(filepath.Join(absFolder, "go.sum"), origGoSum, 0o600)
		} else {
			os.Remove(filepath.Join(absFolder, "go.sum"))
		}
	}()

	// Resolve the drift-sdk (and any other) module dependencies declared in
	// the function's go.mod. This pulls them from the public module proxy.
	command := exec.Command("go", "mod", "tidy") // #nosec G204 — controlled go toolchain invocation
	command.Dir = absFolder
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy error: %w\n%s", err, string(out))
	}

	command = exec.Command( // #nosec G204 — controlled go toolchain invocation
		"go", "build", "-o", "app",
	)
	command.Dir = absFolder
	command.Env = append(os.Environ(), "GOOS=linux", "CGO_ENABLED=0")
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("go build error: %w\n%s", err, string(out))
	}
	defer os.Remove(filepath.Join(absFolder, "app"))

	sourcePath = filepath.Join(absFolder, "app")

	if err := sendSourceToOperator(name, method, language, auth, element, sourcePath, envKeys, triggers); err != nil {
		return err
	}

	if !quiet {
		fmt.Printf("✅ Function '%s /%s' deployed successfully!\n", method, name)
	}
	return nil
}

func Deploy() *cobra.Command {
	var element string

	atomicDeployCmd := &cobra.Command{
		Use:     "deploy [function folder]",
		Short:   "Deploy a function endpoint",
		Example: "  drift atomic deploy ./send-email\n  drift atomic deploy ./create-invoice --element billing",
		GroupID: "operations",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return DeployFolder(args[0], element, false)
		},
	}

	atomicDeployCmd.Flags().StringVarP(&element, "element", "e", "", "Group this function under a named element (e.g. --element payments)")
	return atomicDeployCmd
}
