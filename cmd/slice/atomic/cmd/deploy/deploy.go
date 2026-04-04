package atomic_cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	atomic_common "cli/cmd/slice/atomic/common"
	"cli/common"

	"github.com/spf13/cobra"
)

//go:embed default/server_post.txt
var defaultGolangServerPost string

//go:embed default/server_get.txt
var defaultGolangServerGet string

//go:embed default/server_ws.txt
var defaultGolangServerWS string

//go:embed default/server_post.py
var defaultPythonServerPost string

//go:embed default/server_get.py
var defaultPythonServerGet string

//go:embed default/server_ws.py
var defaultPythonServerWS string

func generateMain(dir, funcName, route, method string, port int) error {
	var code string
	replacer := strings.NewReplacer(
		"{{FUNC}}", funcName,
		"{{ROUTE}}", route,
		"{{PORT}}", fmt.Sprintf("%d", port),
	)
	switch method {
	case "post":
		code = replacer.Replace(defaultGolangServerPost)
	case "get":
		code = replacer.Replace(defaultGolangServerGet)
	case "ws":
		code = replacer.Replace(defaultGolangServerWS)
	}

	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0o600)
}

// detectLanguage returns "python" if the folder contains handler.py, else "golang".
func detectLanguage(folder string) string {
	if _, err := os.Stat(filepath.Join(folder, "handler.py")); err == nil {
		return "python"
	}
	return "golang"
}

// generatePythonServer writes server.py into dir from the appropriate template.
func generatePythonServer(dir, funcName, name, method, auth, element string) error {
	var code string
	replacer := strings.NewReplacer(
		"{{FUNC}}", funcName,
		"{{FUNCTION_NAME}}", name,
		"{{FUNCTION_METHOD}}", method,
		"{{FUNCTION_AUTH}}", auth,
		"{{FUNCTION_ELEMENT}}", element,
	)
	switch strings.ToLower(method) {
	case "post":
		code = replacer.Replace(defaultPythonServerPost)
	case "get":
		code = replacer.Replace(defaultPythonServerGet)
	case "ws":
		code = replacer.Replace(defaultPythonServerWS)
	}
	return os.WriteFile(filepath.Join(dir, "server.py"), []byte(code), 0o600)
}

// zipPythonFolder zips handler.py, server.py, requirements.txt, and the vendor/
// directory (if present) into a single archive written to destPath.
func zipPythonFolder(folder, destPath string) error {
	out, err := os.Create(destPath) // #nosec G304 — CLI tool creates temp zip by design
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	include := []string{"handler.py", "server.py", "requirements.txt"}
	for _, name := range include {
		src := filepath.Join(folder, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		if err := addFileToZip(zw, src, name); err != nil {
			return err
		}
	}

	// Add vendor/ directory recursively
	vendorDir := filepath.Join(folder, "vendor")
	if _, err := os.Stat(vendorDir); err == nil {
		err = filepath.Walk(vendorDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(folder, path)
			return addFileToZip(zw, path, rel)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, zipPath string) error {
	data, err := os.ReadFile(srcPath) // #nosec G304 — CLI tool reads user files by design
	if err != nil {
		return err
	}
	w, err := zw.Create(zipPath)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
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
	listReq, err := common.NewAuthenticatedRequest(http.MethodGet, "http://api.localhost:30036/ops/backbone/secret/list", nil)
	if err != nil {
		return nil, fmt.Errorf("not logged in: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(listReq)
	if err != nil {
		return nil, fmt.Errorf("could not reach API: %w", err)
	}
	defer resp.Body.Close()

	var existing []string
	if resp.StatusCode == http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
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
		fmt.Printf("🔐 Found %d secret(s) in .env not yet in Backbone: %s\n", len(missing), strings.Join(missing, ", "))
		fmt.Print("   Push them to Backbone now? [Y/n] ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			for _, k := range missing {
				body, _ := json.Marshal(map[string]string{"name": k, "value": pairs[k]})
				req, err := common.NewAuthenticatedRequest(http.MethodPost, "http://api.localhost:30036/ops/backbone/secret/set", bytes.NewBuffer(body))
				if err != nil {
					return nil, fmt.Errorf("not logged in: %w", err)
				}
				req.Header.Set("Content-Type", "application/json")
				r, err := client.Do(req)
				if err != nil {
					fmt.Printf("   ❌ Failed to push %s: %v\n", k, err)
					continue
				}
				r.Body.Close()
				if r.StatusCode == http.StatusNoContent {
					fmt.Printf("   ✅ %s pushed to Backbone\n", k)
				} else {
					fmt.Printf("   ❌ %s: API returned %d\n", k, r.StatusCode)
				}
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

// sendSourceToOperator streams the compiled binary/zip to the API as
// multipart/form-data — no base64 encoding, no heap-resident copy of the file.
func sendSourceToOperator(name, method, language, element, sourcePath string, envKeys []string, triggers []TriggerSpec) error {
	meta, err := json.Marshal(map[string]any{
		"name":     name,
		"method":   method,
		"language": language,
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

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		metaPart, err := mw.CreateFormField("metadata")
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := metaPart.Write(meta); err != nil {
			pw.CloseWithError(err)
			return
		}
		srcPart, err := mw.CreateFormFile("source", filepath.Base(sourcePath))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(srcPart, f); err != nil {
			pw.CloseWithError(err)
			return
		}
		mw.Close()
		pw.Close()
	}()

	req, err := common.NewAuthenticatedRequest("POST", "http://api.localhost:30036/ops/atomic", pr)
	if err != nil {
		pw.CloseWithError(err)
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deploy rejected (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// DeployFolder builds and deploys the atomic function at folder. It is
// exported so that "drift deploy" can call it directly without going through
// the cobra command layer.
func DeployFolder(folder, element string) error {
	method, name, auth, err := atomic_common.ParseAtomicMetadataFromDir(folder)
	if err != nil {
		return fmt.Errorf("failed to parse Atomic metadata: %w", err)
	}
	if element != "" {
		fmt.Printf("🚀 Deploying function '%s /%s/%s' (auth: %s, element: %s)\n", method, element, name, auth, element)
	} else {
		fmt.Printf("🚀 Deploying function '%s /%s' (auth: %s)\n", method, name, auth)
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
	if len(triggers) > 0 {
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

	lang := detectLanguage(absFolder)
	namePascal := ""
	for _, seg := range strings.Split(name, "-") {
		namePascal += common.CapitalizeFirst(strings.ToLower(seg))
	}
	funcName := common.CapitalizeFirst(strings.ToLower(method)) + namePascal

	var sourcePath, language string

	if lang == "python" {
		language = "python"

		if err := generatePythonServer(absFolder, funcName, name, method, auth, element); err != nil {
			return fmt.Errorf("failed to generate server.py: %w", err)
		}

		reqPath := filepath.Join(absFolder, "requirements.txt")
		if _, err := os.Stat(reqPath); err == nil {
			fmt.Println("Installing Python dependencies...")
			pip := exec.Command( // #nosec G204 — controlled pip invocation
				"pip3", "install", "-r", "requirements.txt",
				"--target", "vendor", "--quiet",
			)
			pip.Dir = absFolder
			if out, err := pip.CombinedOutput(); err != nil {
				log.Printf("pip install error: %v, output: %s", err, string(out))
			}
		}

		zipPath := filepath.Join(absFolder, "source.zip")
		if err := zipPythonFolder(absFolder, zipPath); err != nil {
			return fmt.Errorf("failed to create source zip: %w", err)
		}
		defer os.Remove(zipPath)
		defer os.Remove(filepath.Join(absFolder, "server.py"))

		sourcePath = zipPath
	} else {
		language = "golang"

		if err := generateMain(absFolder, funcName, "/"+name, method, 8080); err != nil {
			return fmt.Errorf("failed to generate main.go: %w", err)
		}
		defer os.Remove(filepath.Join(absFolder, "main.go"))

		command := exec.Command( // #nosec G204 — controlled go toolchain invocation
			"go", "mod", "download",
			fmt.Sprintf("-modfile=%s/go.mod", absFolder),
		)
		command.Dir = absFolder
		if out, err := command.CombinedOutput(); err != nil {
			fmt.Printf("Error running go mod download: %v\nOutput: %s\n", err, string(out))
		}

		command = exec.Command( // #nosec G204 — controlled go toolchain invocation
			"go", "build",
			"-ldflags", fmt.Sprintf("-X main.functionName=%s -X main.functionMethod=%s -X main.functionAuth=%s -X main.functionElement=%s", name, method, auth, element),
			"-o", "app",
		)
		command.Dir = absFolder
		command.Env = append(os.Environ(), "GOOS=linux")
		if out, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("go build error: %w\n%s", err, string(out))
		}
		defer os.Remove(filepath.Join(absFolder, "app"))

		sourcePath = filepath.Join(absFolder, "app")
	}

	if err := sendSourceToOperator(name, method, language, element, sourcePath, envKeys, triggers); err != nil {
		return err
	}

	fmt.Printf("✅ Function '%s /%s' deployed successfully!\n", method, name)
	return nil
}

func Deploy() *cobra.Command {
	var element string

	atomicDeployCmd := &cobra.Command{
		Use:     "deploy [function folder]",
		Short:   "Deploy a function endpoint",
		GroupID: "operations",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return DeployFolder(args[0], element)
		},
	}

	atomicDeployCmd.Flags().StringVarP(&element, "element", "e", "", "Group this function under a named element (e.g. --element payments)")
	return atomicDeployCmd
}
