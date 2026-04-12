package atomic_cmd

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
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
	"runtime"
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

//go:embed default/wrapper_post_python.txt
var wrapperPostPython string

//go:embed default/wrapper_get_python.txt
var wrapperGetPython string

//go:embed default/wrapper_post_node.txt
var wrapperPostNode string

//go:embed default/wrapper_get_node.txt
var wrapperGetNode string

//go:embed default/drift_sdk.py
var embeddedPythonSDK string

//go:embed default/drift_sdk_node.js
var embeddedNodeSDK string

//go:embed default/wrapper_post_ruby.txt
var wrapperPostRuby string

//go:embed default/wrapper_get_ruby.txt
var wrapperGetRuby string

//go:embed default/wrapper_post_php.txt
var wrapperPostPHP string

//go:embed default/wrapper_get_php.txt
var wrapperGetPHP string

//go:embed default/wrapper_post_rust.txt
var wrapperPostRust string

//go:embed default/wrapper_get_rust.txt
var wrapperGetRust string

//go:embed default/drift_sdk_ruby.rb
var embeddedRubySDK string

//go:embed default/drift_sdk_php.php
var embeddedPHPSDK string

//go:embed default/drift_sdk_rust.rs
var embeddedRustSDK string

//go:embed default/cargo_template.toml
var cargoTemplate string

// generateMain writes a main.go that wraps the user's Go handler function.
func generateMain(dir, funcName, method string) error {
	var code string
	replacer := strings.NewReplacer("{{FUNC}}", funcName)
	switch method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(defaultNativeServerPost)
	default:
		code = replacer.Replace(defaultNativeServerGet)
	}
	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0o600)
}

// generatePythonWrapper writes app.py that wraps the user's Python function.
func generatePythonWrapper(dir, sourceModule, funcName, method string) error {
	replacer := strings.NewReplacer("{{FUNC}}", funcName, "{{SOURCE}}", sourceModule)
	var code string
	switch method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(wrapperPostPython)
	default:
		code = replacer.Replace(wrapperGetPython)
	}
	return os.WriteFile(filepath.Join(dir, "app.py"), []byte(code), 0o600)
}

// generateNodeWrapper writes app.js that wraps the user's Node function.
func generateNodeWrapper(dir, sourceModule, funcName, method string) error {
	replacer := strings.NewReplacer("{{FUNC}}", funcName, "{{SOURCE}}", sourceModule)
	var code string
	switch method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(wrapperPostNode)
	default:
		code = replacer.Replace(wrapperGetNode)
	}
	return os.WriteFile(filepath.Join(dir, "app.js"), []byte(code), 0o600)
}

// generateRubyWrapper writes app.rb that wraps the user's Ruby function.
func generateRubyWrapper(dir, sourceModule, funcName, method string) error {
	replacer := strings.NewReplacer("{{FUNC}}", funcName, "{{SOURCE}}", sourceModule)
	var code string
	switch method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(wrapperPostRuby)
	default:
		code = replacer.Replace(wrapperGetRuby)
	}
	return os.WriteFile(filepath.Join(dir, "app.rb"), []byte(code), 0o600)
}

// generatePHPWrapper writes app.php that wraps the user's PHP function.
func generatePHPWrapper(dir, sourceModule, funcName, method string) error {
	replacer := strings.NewReplacer("{{FUNC}}", funcName, "{{SOURCE}}", sourceModule)
	var code string
	switch method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(wrapperPostPHP)
	default:
		code = replacer.Replace(wrapperGetPHP)
	}
	return os.WriteFile(filepath.Join(dir, "app.php"), []byte(code), 0o600)
}

// generateRustWrapper writes src/main.rs that wraps the user's Rust function.
func generateRustWrapper(dir, sourceModule, funcName, method string) error {
	replacer := strings.NewReplacer("{{FUNC}}", funcName, "{{SOURCE}}", sourceModule)
	var code string
	switch method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(wrapperPostRust)
	default:
		code = replacer.Replace(wrapperGetRust)
	}
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(code), 0o600)
}


// createTarGz creates a .tar.gz archive of the given directory, writing it to destPath.
// Only regular files and directories are included. Hidden files (.git etc) are skipped.
func createTarGz(srcDir, destPath string) error {
	f, err := os.Create(destPath) // #nosec G304 — CLI creates temp archive
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip hidden directories/files.
		base := filepath.Base(rel)
		if strings.HasPrefix(base, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		src, err := os.Open(path) // #nosec G304 — CLI reads user's source files
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
}

// TriggerSpec is the minimal trigger definition declared in source comments.
type TriggerSpec struct {
	Type     string `json:"type"`               // "queue" | "webhook" | "schedule"
	Source   string `json:"source"`             // queue name or webhook path
	Schedule string `json:"schedule,omitempty"` // Go duration string e.g. "5m" (schedule triggers)
	PollMS   int    `json:"poll_ms,omitempty"`  // polling interval ms (queue triggers)
	MaxRetry int    `json:"max_retry,omitempty"`
}

// sourceFiles returns all source files (.go, .py, .js, .rb, .php, .rs) in dir.
func sourceFiles(dir string) []string {
	var files []string
	for _, ext := range []string{"*.go", "*.py", "*.js", "*.rb", "*.php", "*.rs"} {
		matches, _ := filepath.Glob(filepath.Join(dir, ext))
		files = append(files, matches...)
	}
	return files
}

// extractTriggerLine extracts the content after "drift:trigger " from a comment line.
// Supports both // and # comment prefixes.
func extractTriggerLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"// drift:trigger ", "# drift:trigger "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix), true
		}
	}
	return "", false
}

// extractScheduleLine extracts the content after "drift:schedule " from a comment line.
func extractScheduleLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"// drift:schedule ", "# drift:schedule "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

// parseTriggerComments scans source files in dir for drift:trigger annotations.
//
// Supported formats (// for Go/Node, # for Python):
//
//	// drift:trigger queue my-queue
//	// drift:trigger queue my-queue poll=250ms retry=5
//	# drift:trigger webhook /hooks/payment
func parseTriggerComments(dir string) ([]TriggerSpec, error) {
	var triggers []TriggerSpec
	for _, f := range sourceFiles(dir) {
		data, err := os.ReadFile(f) // #nosec G304 — CLI reads user's source file by design
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			content, ok := extractTriggerLine(line)
			if !ok {
				continue
			}
			parts := strings.Fields(content)
			if len(parts) < 2 {
				continue
			}
			trigType := parts[0]
			if trigType != "queue" && trigType != "webhook" {
				fmt.Printf("Warning: unknown trigger type %q — skipping\n", trigType)
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

// parseScheduleComments scans source files in dir for drift:schedule annotations.
//
// The value must be a standard 5-field cron expression (minute hour dom month dow).
//
//	// drift:schedule */5 * * * *
//	# drift:schedule 0 15 * * *
func parseScheduleComments(dir string) ([]TriggerSpec, error) {
	var triggers []TriggerSpec
	for _, f := range sourceFiles(dir) {
		data, err := os.ReadFile(f) // #nosec G304 — CLI reads user's source file by design
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			expr, ok := extractScheduleLine(line)
			if !ok || expr == "" {
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

	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return fmt.Errorf("failed to resolve folder path: %w", err)
	}

	language, _, err := atomic_common.DetectLanguage(absFolder)
	if err != nil {
		return fmt.Errorf("failed to detect language: %w", err)
	}

	if !quiet {
		langLabel := language
		if langLabel == "native" {
			langLabel = "go"
		}
		if element != "" {
			fmt.Printf("Deploying function '%s /%s/%s' (%s, auth: %s)\n", method, element, name, langLabel, auth)
		} else {
			fmt.Printf("Deploying function '%s /%s' (%s, auth: %s)\n", method, name, langLabel, auth)
		}
	}

	envKeys, err := syncEnvToBackbone(absFolder)
	if err != nil {
		return fmt.Errorf("env sync failed: %w", err)
	}

	triggers, err := parseTriggerComments(absFolder)
	if err != nil {
		return fmt.Errorf("failed to parse trigger comments: %w", err)
	}
	schedules, err := parseScheduleComments(absFolder)
	if err != nil {
		return fmt.Errorf("failed to parse schedule comments: %w", err)
	}
	triggers = append(triggers, schedules...)
	if len(triggers) > 0 && !quiet {
		fmt.Printf("  %d trigger(s) found: ", len(triggers))
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

	var sourcePath string
	switch language {
	case "python":
		sourcePath, err = buildPython(absFolder, method, name)
	case "node":
		sourcePath, err = buildNode(absFolder, method, name)
	case "ruby":
		sourcePath, err = buildRuby(absFolder, method, name)
	case "php":
		sourcePath, err = buildPHP(absFolder, method, name)
	case "rust":
		sourcePath, err = buildRust(absFolder, method, name)
	default:
		sourcePath, err = buildGo(absFolder, method, name)
	}
	if err != nil {
		return err
	}
	defer os.Remove(sourcePath)

	if err := sendSourceToOperator(name, method, language, auth, element, sourcePath, envKeys, triggers); err != nil {
		return err
	}

	if !quiet {
		fmt.Printf("  Function '%s /%s' deployed successfully!\n", method, name)
	}
	return nil
}

// buildGo compiles a Go function to a static Linux binary and returns the path.
func buildGo(absFolder, method, name string) (string, error) {
	funcName := atomic_common.FuncNameForLanguage(method, name, "native")

	if err := generateMain(absFolder, funcName, method); err != nil {
		return "", fmt.Errorf("failed to generate main.go: %w", err)
	}
	defer os.Remove(filepath.Join(absFolder, "main.go"))

	// Back up go.mod/go.sum so `go mod tidy` doesn't dirty the user's working tree.
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

	cmd := exec.Command("go", "mod", "tidy") // #nosec G204
	cmd.Dir = absFolder
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go mod tidy error: %w\n%s", err, string(out))
	}

	cmd = exec.Command("go", "build", "-o", "app") // #nosec G204
	cmd.Dir = absFolder
	cmd.Env = append(os.Environ(), "GOOS=linux", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build error: %w\n%s", err, string(out))
	}

	return filepath.Join(absFolder, "app"), nil
}

// buildPython generates the wrapper, installs deps, injects the SDK,
// creates a tar.gz archive, and returns its path.
func buildPython(absFolder, method, name string) (string, error) {
	funcName := atomic_common.FuncNameForLanguage(method, name, "python")

	// Find the user's source file (the .py with @atomic annotation).
	_, sourceFile, err := atomic_common.DetectLanguage(absFolder)
	if err != nil {
		return "", err
	}
	sourceModule := strings.TrimSuffix(filepath.Base(sourceFile), ".py")

	// Create a staging directory for the archive.
	stageDir, err := os.MkdirTemp("", "drift-python-")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	// Copy all .py files from the function directory.
	entries, _ := os.ReadDir(absFolder)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(absFolder, e.Name()))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(stageDir, e.Name()), data, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}

	// Generate wrapper app.py.
	if err := generatePythonWrapper(stageDir, sourceModule, funcName, method); err != nil {
		return "", fmt.Errorf("generate wrapper: %w", err)
	}

	// Inject the drift SDK.
	if err := os.WriteFile(filepath.Join(stageDir, "drift.py"), []byte(embeddedPythonSDK), 0o644); err != nil {
		return "", fmt.Errorf("inject SDK: %w", err)
	}

	// Install dependencies from requirements.txt if present.
	// Use --platform to download Linux wheels (container target) instead of host OS wheels.
	reqPath := filepath.Join(absFolder, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		vendorDir := filepath.Join(stageDir, "vendor")
		pipPlatform := "manylinux2014_x86_64"
		if runtime.GOARCH == "arm64" {
			pipPlatform = "manylinux2014_aarch64"
		}
		cmd := exec.Command("pip3", "install", // #nosec G204
			"-t", vendorDir,
			"-r", reqPath,
			"--platform", pipPlatform,
			"--python-version", "3.11",
			"--only-binary=:all:",
			"--quiet",
		)
		cmd.Dir = absFolder
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("pip install error: %w\n%s", err, string(out))
		}
	}

	// Create tar.gz archive.
	archivePath := filepath.Join(os.TempDir(), fmt.Sprintf("drift-python-%s.tar.gz", name))
	if err := createTarGz(stageDir, archivePath); err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}

	return archivePath, nil
}

// buildNode generates the wrapper, installs deps, injects the SDK,
// creates a tar.gz archive, and returns its path.
func buildNode(absFolder, method, name string) (string, error) {
	funcName := atomic_common.FuncNameForLanguage(method, name, "node")

	// Find the user's source file.
	_, sourceFile, err := atomic_common.DetectLanguage(absFolder)
	if err != nil {
		return "", err
	}
	sourceModule := strings.TrimSuffix(filepath.Base(sourceFile), ".js")

	// Create a staging directory.
	stageDir, err := os.MkdirTemp("", "drift-node-")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	// Copy all .js files from the function directory.
	entries, _ := os.ReadDir(absFolder)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(absFolder, e.Name()))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(stageDir, e.Name()), data, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}

	// Generate wrapper app.js.
	if err := generateNodeWrapper(stageDir, sourceModule, funcName, method); err != nil {
		return "", fmt.Errorf("generate wrapper: %w", err)
	}

	// Inject the drift SDK as drift-sdk.js (wrapper requires('./drift-sdk')).
	if err := os.WriteFile(filepath.Join(stageDir, "drift-sdk.js"), []byte(embeddedNodeSDK), 0o644); err != nil {
		return "", fmt.Errorf("inject SDK: %w", err)
	}

	// Install dependencies from package.json if present.
	pkgPath := filepath.Join(absFolder, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		// Copy package.json and package-lock.json to staging dir.
		data, _ := os.ReadFile(pkgPath)
		os.WriteFile(filepath.Join(stageDir, "package.json"), data, 0o644)
		if lockData, err := os.ReadFile(filepath.Join(absFolder, "package-lock.json")); err == nil {
			os.WriteFile(filepath.Join(stageDir, "package-lock.json"), lockData, 0o644)
		}

		// Use --os and --cpu to resolve Linux platform-specific optional dependencies
		// (e.g. @img/sharp-linux-arm64) instead of host OS binaries.
		npmCPU := "x64"
		if runtime.GOARCH == "arm64" {
			npmCPU = "arm64"
		}
		cmd := exec.Command("npm", "install", "--production", "--silent", "--os=linux", "--cpu="+npmCPU) // #nosec G204
		cmd.Dir = stageDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("npm install error: %w\n%s", err, string(out))
		}
	}

	// Create tar.gz archive.
	archivePath := filepath.Join(os.TempDir(), fmt.Sprintf("drift-node-%s.tar.gz", name))
	if err := createTarGz(stageDir, archivePath); err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}

	return archivePath, nil
}

// buildRuby generates the wrapper, installs deps, injects the SDK,
// creates a tar.gz archive, and returns its path.
func buildRuby(absFolder, method, name string) (string, error) {
	funcName := atomic_common.FuncNameForLanguage(method, name, "ruby")

	_, sourceFile, err := atomic_common.DetectLanguage(absFolder)
	if err != nil {
		return "", err
	}
	sourceModule := strings.TrimSuffix(filepath.Base(sourceFile), ".rb")

	stageDir, err := os.MkdirTemp("", "drift-ruby-")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	// Copy all .rb files.
	entries, _ := os.ReadDir(absFolder)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rb") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(absFolder, e.Name()))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(stageDir, e.Name()), data, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}

	if err := generateRubyWrapper(stageDir, sourceModule, funcName, method); err != nil {
		return "", fmt.Errorf("generate wrapper: %w", err)
	}

	if err := os.WriteFile(filepath.Join(stageDir, "drift.rb"), []byte(embeddedRubySDK), 0o644); err != nil {
		return "", fmt.Errorf("inject SDK: %w", err)
	}

	// Install gems if Gemfile is present.
	// Use Docker to run bundle install inside a Linux container so that native gems
	// (e.g. nokogiri) get the correct platform binaries for the container target.
	gemfilePath := filepath.Join(absFolder, "Gemfile")
	if _, err := os.Stat(gemfilePath); err == nil {
		data, _ := os.ReadFile(gemfilePath)
		os.WriteFile(filepath.Join(stageDir, "Gemfile"), data, 0o644)
		if lockData, err := os.ReadFile(filepath.Join(absFolder, "Gemfile.lock")); err == nil {
			os.WriteFile(filepath.Join(stageDir, "Gemfile.lock"), lockData, 0o644)
		}

		cmd := exec.Command("docker", "run", "--rm", // #nosec G204
			"-v", stageDir+":/app",
			"-w", "/app",
			"ruby:3.1",
			"sh", "-c",
			"bundle config set --local path vendor/bundle && "+
				"bundle config set --local without development:test && "+
				"bundle install --standalone --quiet",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("bundle install error: %w\n%s", err, string(out))
		}
	}

	archivePath := filepath.Join(os.TempDir(), fmt.Sprintf("drift-ruby-%s.tar.gz", name))
	if err := createTarGz(stageDir, archivePath); err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}

	return archivePath, nil
}

// buildPHP generates the wrapper, installs deps, injects the SDK,
// creates a tar.gz archive, and returns its path.
func buildPHP(absFolder, method, name string) (string, error) {
	funcName := atomic_common.FuncNameForLanguage(method, name, "php")

	_, sourceFile, err := atomic_common.DetectLanguage(absFolder)
	if err != nil {
		return "", err
	}
	sourceModule := strings.TrimSuffix(filepath.Base(sourceFile), ".php")

	stageDir, err := os.MkdirTemp("", "drift-php-")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	// Copy all .php files.
	entries, _ := os.ReadDir(absFolder)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".php") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(absFolder, e.Name()))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(stageDir, e.Name()), data, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}

	if err := generatePHPWrapper(stageDir, sourceModule, funcName, method); err != nil {
		return "", fmt.Errorf("generate wrapper: %w", err)
	}

	if err := os.WriteFile(filepath.Join(stageDir, "drift.php"), []byte(embeddedPHPSDK), 0o644); err != nil {
		return "", fmt.Errorf("inject SDK: %w", err)
	}

	// Install composer deps if composer.json is present.
	composerPath := filepath.Join(absFolder, "composer.json")
	if _, err := os.Stat(composerPath); err == nil {
		data, _ := os.ReadFile(composerPath)
		os.WriteFile(filepath.Join(stageDir, "composer.json"), data, 0o644)
		if lockData, err := os.ReadFile(filepath.Join(absFolder, "composer.lock")); err == nil {
			os.WriteFile(filepath.Join(stageDir, "composer.lock"), lockData, 0o644)
		}

		cmd := exec.Command("composer", "install", "--no-dev", "--ignore-platform-reqs", "--quiet", "--no-interaction") // #nosec G204
		cmd.Dir = stageDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("composer install error: %w\n%s", err, string(out))
		}
	}

	archivePath := filepath.Join(os.TempDir(), fmt.Sprintf("drift-php-%s.tar.gz", name))
	if err := createTarGz(stageDir, archivePath); err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}

	return archivePath, nil
}

// buildRust compiles the Rust function to a static Linux binary client-side
// and returns the path to the binary.
func buildRust(absFolder, method, name string) (string, error) {
	funcName := atomic_common.FuncNameForLanguage(method, name, "rust")

	_, sourceFile, err := atomic_common.DetectLanguage(absFolder)
	if err != nil {
		return "", err
	}
	sourceModule := strings.TrimSuffix(filepath.Base(sourceFile), ".rs")

	stageDir, err := os.MkdirTemp("", "drift-rust-")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	srcDir := filepath.Join(stageDir, "src")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		return "", fmt.Errorf("create src dir: %w", err)
	}

	// Write Cargo.toml. If user has one, merge deps; otherwise use template.
	userCargoPath := filepath.Join(absFolder, "Cargo.toml")
	if _, err := os.Stat(userCargoPath); err == nil {
		data, _ := os.ReadFile(userCargoPath)
		os.WriteFile(filepath.Join(stageDir, "Cargo.toml"), data, 0o644)
	} else {
		os.WriteFile(filepath.Join(stageDir, "Cargo.toml"), []byte(cargoTemplate), 0o644)
	}

	// Copy all .rs files into src/.
	entries, _ := os.ReadDir(absFolder)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rs") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(absFolder, e.Name()))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, e.Name()), data, 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", e.Name(), err)
		}
	}

	// Generate main.rs wrapper.
	if err := generateRustWrapper(stageDir, sourceModule, funcName, method); err != nil {
		return "", fmt.Errorf("generate wrapper: %w", err)
	}

	// Inject the drift SDK as src/drift.rs.
	if err := os.WriteFile(filepath.Join(srcDir, "drift.rs"), []byte(embeddedRustSDK), 0o644); err != nil {
		return "", fmt.Errorf("inject SDK: %w", err)
	}

	// Build for Linux (cross-compile). Match the host arch since Rancher Desktop
	// runs containers with the same architecture as the host.
	target := "x86_64-unknown-linux-musl"
	if runtime.GOARCH == "arm64" {
		target = "aarch64-unknown-linux-musl"
	}
	cmd := exec.Command("cargo", "build", "--release", "--target", target) // #nosec G204
	cmd.Dir = stageDir
	cmd.Env = os.Environ()
	// Tell Cargo to use the musl cross-linker and CC instead of the host cc.
	// ring's build script needs CC, and rustc needs the target linker.
	linkerEnv := fmt.Sprintf("CARGO_TARGET_%s_LINKER", strings.ReplaceAll(strings.ToUpper(target), "-", "_"))
	crossGCC := strings.TrimSuffix(target, "-musl") + "-musl-gcc" // e.g. aarch64-linux-musl-gcc
	ccEnv := fmt.Sprintf("CC_%s", strings.ReplaceAll(target, "-", "_"))
	cmd.Env = append(cmd.Env, linkerEnv+"="+crossGCC, ccEnv+"="+crossGCC)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("cargo build error (target %s): %w\n%s\nHint: run 'rustup target add %s'", target, err, string(out), target)
	}

	binaryPath := filepath.Join(stageDir, "target", target, "release", "atomic-function")
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("drift-rust-%s", name))
	data, err := os.ReadFile(binaryPath) // #nosec G304
	if err != nil {
		return "", fmt.Errorf("read compiled binary: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o755); err != nil {
		return "", fmt.Errorf("write binary: %w", err)
	}

	return outputPath, nil
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
