package atomic_cmd

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	atomic_common "cli/cmd/atomic/common"
	"cli/common"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// ==========================================================
// Embedded templates: Go server + Python/Node wrappers + SDKs
// ==========================================================

//go:embed default/server_post.txt
var defaultGolangServerPost string

//go:embed default/server_get.txt
var defaultGolangServerGet string

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

// ==========================================================
// Public command factory
// ==========================================================

func Run() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run [function folder]",
		Short:   "Run an Atomic function locally with hot reload",
		Example: "  drift atomic run ./send-email\n  drift atomic run ./create-invoice",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			srcDir := args[0]
			absSrc, err := filepath.Abs(srcDir)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			// Parse metadata (method, name, auth, env)
			method, name, _, err := atomic_common.ParseAtomicMetadataFromDir(absSrc)
			if err != nil {
				return fmt.Errorf("parse Atomic metadata: %w", err)
			}

			// Detect language from the annotated source file.
			language, sourceFile, err := atomic_common.DetectLanguage(absSrc)
			if err != nil {
				return fmt.Errorf("detect language: %w", err)
			}

			// Try to detect port from annotation `port=XXXX` (optional). Falls back to 3000.
			port := detectPortFromAnnotation(absSrc)
			if port == 0 {
				port = 3000
			}

			fmt.Printf("▶️  Running %s function '%s /%s' on http://localhost:%d\n", language, strings.ToUpper(method), name, port)

			// Create a persistent temp workspace for this run session
			workDir, err := os.MkdirTemp("", "drift_atomic_run_*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			// Clean up on exit
			defer os.RemoveAll(workDir)

			// Initial sync & start
			runner := &devRunner{
				srcDir:     absSrc,
				workDir:    workDir,
				method:     strings.ToLower(method),
				name:       name,
				port:       port,
				language:   language,
				sourceFile: sourceFile,
				procLock:   &sync.Mutex{},
			}

			if err := runner.syncWorkspace(); err != nil {
				return err
			}
			if err := runner.generateMain(); err != nil {
				return err
			}
			if err := runner.installDeps(); err != nil {
				log.Printf("dependency install warning: %v", err)
			}
			if err := runner.buildAndRun(); err != nil {
				log.Printf("initial build failed, staying in watch mode: %v", err)
			}

			// Start watching for changes (recursive)
			return runner.watchAndReload()
		},
	}

	return cmd
}

// ==========================================================
// devRunner orchestrates sync → generate → (install deps) → build → run → reload
// ==========================================================

type devRunner struct {
	srcDir     string
	workDir    string
	method     string // get/post/...
	name       string // route/resource name
	port       int
	language   string // "native", "python", "node", "ruby", "php", "rust"
	sourceFile string // filename with extension (e.g., "checkout.py")

	proc     *exec.Cmd
	procLock *sync.Mutex
}

// ---------- generateMain ----------

func (r *devRunner) generateMain() error {
	switch r.language {
	case "python":
		return r.generatePython()
	case "node":
		return r.generateNode()
	case "ruby":
		return r.generateRuby()
	case "php":
		return r.generatePHP()
	case "rust":
		return r.generateRust()
	default:
		return r.generateGo()
	}
}

func (r *devRunner) generateGo() error {
	funcName := common.CapitalizeFirst(strings.ToLower(r.method)) + common.CapitalizeFirst(strings.ToLower(r.name))

	var code string
	replacer := strings.NewReplacer(
		"{{FUNC}}", funcName,
		"{{ROUTE}}", "/"+r.name,
		"{{PORT}}", fmt.Sprintf("%d", r.port),
	)
	switch r.method {
	case "post", "put", "delete", "patch":
		code = replacer.Replace(defaultGolangServerPost)
	default:
		code = replacer.Replace(defaultGolangServerGet)
	}

	return os.WriteFile(filepath.Join(r.workDir, "main.go"), []byte(code), 0o600)
}

func (r *devRunner) generatePython() error {
	funcName := atomic_common.FuncNameForLanguage(r.method, r.name, "python")
	sourceModule := strings.TrimSuffix(r.sourceFile, ".py")

	var tmpl string
	switch r.method {
	case "post", "put", "delete", "patch":
		tmpl = wrapperPostPython
	default:
		tmpl = wrapperGetPython
	}

	code := strings.NewReplacer(
		"{{SOURCE}}", sourceModule,
		"{{FUNC}}", funcName,
	).Replace(tmpl)

	if err := os.WriteFile(filepath.Join(r.workDir, "app.py"), []byte(code), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.workDir, "drift.py"), []byte(embeddedPythonSDK), 0o644)
}

func (r *devRunner) generateNode() error {
	funcName := atomic_common.FuncNameForLanguage(r.method, r.name, "node")
	sourceModule := strings.TrimSuffix(r.sourceFile, ".js")

	var tmpl string
	switch r.method {
	case "post", "put", "delete", "patch":
		tmpl = wrapperPostNode
	default:
		tmpl = wrapperGetNode
	}

	code := strings.NewReplacer(
		"{{SOURCE}}", sourceModule,
		"{{FUNC}}", funcName,
	).Replace(tmpl)

	if err := os.WriteFile(filepath.Join(r.workDir, "app.js"), []byte(code), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.workDir, "drift-sdk.js"), []byte(embeddedNodeSDK), 0o644)
}

func (r *devRunner) generateRuby() error {
	funcName := atomic_common.FuncNameForLanguage(r.method, r.name, "ruby")
	sourceModule := strings.TrimSuffix(r.sourceFile, ".rb")

	var tmpl string
	switch r.method {
	case "post", "put", "delete", "patch":
		tmpl = wrapperPostRuby
	default:
		tmpl = wrapperGetRuby
	}

	code := strings.NewReplacer(
		"{{SOURCE}}", sourceModule,
		"{{FUNC}}", funcName,
	).Replace(tmpl)

	if err := os.WriteFile(filepath.Join(r.workDir, "app.rb"), []byte(code), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.workDir, "drift.rb"), []byte(embeddedRubySDK), 0o644)
}

func (r *devRunner) generatePHP() error {
	funcName := atomic_common.FuncNameForLanguage(r.method, r.name, "php")
	sourceModule := strings.TrimSuffix(r.sourceFile, ".php")

	var tmpl string
	switch r.method {
	case "post", "put", "delete", "patch":
		tmpl = wrapperPostPHP
	default:
		tmpl = wrapperGetPHP
	}

	code := strings.NewReplacer(
		"{{SOURCE}}", sourceModule,
		"{{FUNC}}", funcName,
	).Replace(tmpl)

	if err := os.WriteFile(filepath.Join(r.workDir, "app.php"), []byte(code), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.workDir, "drift.php"), []byte(embeddedPHPSDK), 0o644)
}

func (r *devRunner) generateRust() error {
	funcName := atomic_common.FuncNameForLanguage(r.method, r.name, "rust")
	sourceModule := strings.TrimSuffix(r.sourceFile, ".rs")

	var tmpl string
	switch r.method {
	case "post", "put", "delete", "patch":
		tmpl = wrapperPostRust
	default:
		tmpl = wrapperGetRust
	}

	code := strings.NewReplacer(
		"{{SOURCE}}", sourceModule,
		"{{FUNC}}", funcName,
	).Replace(tmpl)

	srcDir := filepath.Join(r.workDir, "src")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(code), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "drift.rs"), []byte(embeddedRustSDK), 0o644); err != nil {
		return err
	}
	// Move user source into src/ if not already there.
	userSrc := filepath.Join(r.workDir, r.sourceFile)
	userSrcDest := filepath.Join(srcDir, r.sourceFile)
	if _, err := os.Stat(userSrc); err == nil {
		data, _ := os.ReadFile(userSrc)
		os.WriteFile(userSrcDest, data, 0o644)
	}
	// Write Cargo.toml if user doesn't have one.
	cargoPath := filepath.Join(r.workDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err != nil {
		os.WriteFile(cargoPath, []byte(cargoTemplate), 0o644)
	}
	return nil
}

// ---------- installDeps (run once at startup) ----------

func (r *devRunner) installDeps() error {
	switch r.language {
	case "python":
		reqPath := filepath.Join(r.srcDir, "requirements.txt")
		if _, err := os.Stat(reqPath); err != nil {
			return nil
		}
		vendorDir := filepath.Join(r.workDir, "vendor")
		fmt.Println("📦 Installing Python dependencies...")
		return runCmd(r.workDir, "pip3", "install", "-t", vendorDir, "-r", reqPath, "--quiet")

	case "node":
		pkgPath := filepath.Join(r.srcDir, "package.json")
		if _, err := os.Stat(pkgPath); err != nil {
			return nil
		}
		data, err := os.ReadFile(pkgPath) // #nosec G304 — CLI tool reads user's own project files
		if err != nil {
			return err
		}
		_ = os.WriteFile(filepath.Join(r.workDir, "package.json"), data, 0o644)
		if lockData, err := os.ReadFile(filepath.Join(r.srcDir, "package-lock.json")); err == nil {
			_ = os.WriteFile(filepath.Join(r.workDir, "package-lock.json"), lockData, 0o644)
		}
		fmt.Println("📦 Installing Node dependencies...")
		return runCmd(r.workDir, "npm", "install", "--production", "--silent")

	case "ruby":
		gemfilePath := filepath.Join(r.srcDir, "Gemfile")
		if _, err := os.Stat(gemfilePath); err != nil {
			return nil
		}
		data, _ := os.ReadFile(gemfilePath)
		_ = os.WriteFile(filepath.Join(r.workDir, "Gemfile"), data, 0o644)
		if lockData, err := os.ReadFile(filepath.Join(r.srcDir, "Gemfile.lock")); err == nil {
			_ = os.WriteFile(filepath.Join(r.workDir, "Gemfile.lock"), lockData, 0o644)
		}
		fmt.Println("📦 Installing Ruby dependencies...")
		return runCmd(r.workDir, "bundle", "install", "--standalone", "--path", "vendor/bundle", "--without", "development:test", "--quiet")

	case "php":
		composerPath := filepath.Join(r.srcDir, "composer.json")
		if _, err := os.Stat(composerPath); err != nil {
			return nil
		}
		data, _ := os.ReadFile(composerPath)
		_ = os.WriteFile(filepath.Join(r.workDir, "composer.json"), data, 0o644)
		if lockData, err := os.ReadFile(filepath.Join(r.srcDir, "composer.lock")); err == nil {
			_ = os.WriteFile(filepath.Join(r.workDir, "composer.lock"), lockData, 0o644)
		}
		fmt.Println("📦 Installing PHP dependencies...")
		return runCmd(r.workDir, "composer", "install", "--no-dev", "--quiet", "--no-interaction")

	default:
		return nil // Go and Rust deps handled in buildAndRun
	}
}

// ---------- buildAndRun ----------

func (r *devRunner) buildAndRun() error {
	switch r.language {
	case "python":
		return r.runInterpreted("python3", "app.py")
	case "node":
		return r.runInterpreted("node", "app.js")
	case "ruby":
		return r.runInterpreted("ruby", "app.rb")
	case "php":
		return r.runInterpreted("php", "app.php")
	case "rust":
		return r.buildAndRunRust()
	default:
		return r.buildAndRunGo()
	}
}

func (r *devRunner) buildAndRunGo() error {
	// Ensure go.mod/sum deps are available for the temp workspace
	if err := runCmd(r.workDir, "go", "mod", "download"); err != nil {
		// Not fatal; the project may not need it, but log it.
		log.Printf("go mod download warning: %v", err)
	}

	// Build for the local OS/arch (we're running locally)
	buildArgs := []string{"build", "-o", "app"}
	if err := runCmd(r.workDir, "go", buildArgs...); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	// Stop any existing process
	r.stopProcess()

	// Load .env (from source) into environment for the run if present in annotation
	envs, _ := readDotEnv(filepath.Join(r.srcDir, ".env"))
	runEnv := os.Environ()
	runEnv = append(runEnv, envs...)
	runEnv = append(runEnv, fmt.Sprintf("PORT=%d", r.port))

	// Start the process
	cmd := exec.Command(filepath.Join(r.workDir, "app")) // #nosec G204 — path is a controlled temp workspace
	cmd.Dir = r.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = runEnv

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start app: %w", err)
	}

	r.procLock.Lock()
	r.proc = cmd
	r.procLock.Unlock()

	go func() {
		_ = cmd.Wait()
	}()

	fmt.Printf("✅ Server started (PID %d)\n", cmd.Process.Pid)
	return nil
}

func (r *devRunner) buildAndRunRust() error {
	// Build for local OS/arch (not cross-compiling for local dev).
	buildArgs := []string{"build", "--release"}
	if err := runCmd(r.workDir, "cargo", buildArgs...); err != nil {
		return fmt.Errorf("cargo build: %w", err)
	}

	r.stopProcess()

	envs, _ := readDotEnv(filepath.Join(r.srcDir, ".env"))
	runEnv := os.Environ()
	runEnv = append(runEnv, envs...)
	runEnv = append(runEnv, fmt.Sprintf("PORT=%d", r.port))

	binaryPath := filepath.Join(r.workDir, "target", "release", "atomic-function")
	cmd := exec.Command(binaryPath) // #nosec G204 — controlled temp workspace
	cmd.Dir = r.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = runEnv

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start app: %w", err)
	}

	r.procLock.Lock()
	r.proc = cmd
	r.procLock.Unlock()

	go func() {
		_ = cmd.Wait()
	}()

	fmt.Printf("✅ Server started (PID %d)\n", cmd.Process.Pid)
	return nil
}

func (r *devRunner) runInterpreted(interpreter, entryPoint string) error {
	r.stopProcess()

	envs, _ := readDotEnv(filepath.Join(r.srcDir, ".env"))
	runEnv := os.Environ()
	runEnv = append(runEnv, envs...)
	runEnv = append(runEnv, fmt.Sprintf("PORT=%d", r.port))

	cmd := exec.Command(interpreter, entryPoint) // #nosec G204 — controlled interpreter + entry point
	cmd.Dir = r.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = runEnv

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", interpreter, err)
	}

	r.procLock.Lock()
	r.proc = cmd
	r.procLock.Unlock()

	go func() {
		_ = cmd.Wait()
	}()

	fmt.Printf("✅ Server started (PID %d)\n", cmd.Process.Pid)
	return nil
}

// ---------- stop / sync / rebuild / watch ----------

func (r *devRunner) stopProcess() {
	r.procLock.Lock()
	defer r.procLock.Unlock()
	if r.proc != nil && r.proc.Process != nil {
		// Try graceful stop on POSIX
		if runtime.GOOS != "windows" {
			_ = r.proc.Process.Signal(os.Interrupt)
			done := make(chan struct{})
			go func() { _ = r.proc.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(800 * time.Millisecond):
				_ = r.proc.Process.Kill()
			}
		} else {
			_ = r.proc.Process.Kill()
		}
		r.proc = nil
	}
}

func (r *devRunner) syncWorkspace() error {
	// Copy entire srcDir → workDir, filtering out junk. Then we overwrite main/app later.
	filters := []string{`.git`, `.idea`, `.vscode`, `node_modules`, `vendor`, `app`, `bin`, `dist`}
	if err := copyDir(r.srcDir, r.workDir, filters); err != nil {
		return fmt.Errorf("sync workspace: %w", err)
	}
	return nil
}

func (r *devRunner) rebuild() error {
	// Re-sync to pick up any new files/deletions
	if err := r.syncWorkspace(); err != nil {
		return err
	}
	if err := r.generateMain(); err != nil {
		return err
	}
	if err := r.buildAndRun(); err != nil {
		log.Printf("build/run failed: %v", err)
		return err
	}
	return nil
}

func (r *devRunner) watchAndReload() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Recursively watch the srcDir
	if err := addRecursive(watcher, r.srcDir); err != nil {
		return err
	}

	// Debounce changes to avoid burst rebuilds
	var (
		debounceMu sync.Mutex
		debounce   *time.Timer
		trigger    = func() {
			debounceMu.Lock()
			defer debounceMu.Unlock()
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(400*time.Millisecond, func() {
				fmt.Println("\n🔄 Change detected — rebuilding…")
				if err := r.rebuild(); err != nil {
					// keep watching even if rebuild fails
				}
			})
		}
	)

	// Handle Ctrl+C to clean up
	go func() {
		sig := make(chan os.Signal, 1)
		// signal.Notify is platform-dependent; keep it simple
		// (caller process handles SIGINT; we just ensure child is stopped on exit)
		<-sig
		r.stopProcess()
		os.Exit(0)
	}()

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return errors.New("watcher closed")
			}
			// Ignore temporary/editor files
			if isIgnorablePath(ev.Name) {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				trigger()
				// If a new directory was created, start watching it
				if ev.Op&fsnotify.Create != 0 {
					if stat, err := os.Stat(ev.Name); err == nil && stat.IsDir() {
						_ = addRecursive(watcher, ev.Name)
					}
				}
			}
		case err := <-watcher.Errors:
			if err != nil {
				log.Printf("watcher error: %v", err)
			}
		}
	}
}

// ==========================================================
// Utilities
// ==========================================================

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if isFilteredDir(d.Name()) {
				return filepath.SkipDir
			}
			return w.Add(p)
		}
		return nil
	})
}

func isFilteredDir(name string) bool {
	switch name {
	case ".git", ".idea", ".vscode", "node_modules", "vendor", "bin", "dist":
		return true
	default:
		return false
	}
}

func isIgnorablePath(p string) bool {
	base := filepath.Base(p)
	// Common editor swap files and temp artifacts
	ignorable := []string{".DS_Store", "4913"}
	for _, s := range ignorable {
		if base == s {
			return true
		}
	}
	// Vim/Emacs/JetBrains temp files
	if strings.HasPrefix(base, ".#") || strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp") {
		return true
	}
	return false
}

func copyDir(src, dst string, exclude []string) error {
	// Ensure dst exists
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return err
	}

	excluded := map[string]struct{}{}
	for _, e := range exclude {
		excluded[e] = struct{}{}
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		// Skip excluded directories anywhere in the path
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, p := range parts {
			if _, ok := excluded[p]; ok {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		// Copy files
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 — CLI tool reads user's own project files by design
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}

	out, err := os.Create(dst) // #nosec G304 — CLI tool writes to user's own workspace by design
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// Simple .env reader (no external deps). Lines like KEY=VALUE, ignoring comments and empty lines.
func readDotEnv(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 — CLI tool reads user's .env file by design
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var envs []string
	scanner := bufio.NewScanner(f)
	re := regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := re.FindStringSubmatch(line)
		if len(m) == 3 {
			key := m[1]
			val := m[2]
			// strip optional surrounding quotes
			val = strings.Trim(val, "\"'")
			envs = append(envs, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return envs, nil
}

// Attempt to read `port=####` from the annotation line in any source file at src root.
func detectPortFromAnnotation(src string) int {
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".go" && ext != ".py" && ext != ".js" && ext != ".rb" && ext != ".php" && ext != ".rs" {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(src, e.Name())) // #nosec G304 — CLI tool reads user's own source files by design
		line := firstAnnotationLine(string(b))
		if line == "" {
			continue
		}
		// naive parse for `port=XXXX` tokens
		for _, tok := range strings.Fields(line) {
			if strings.HasPrefix(tok, "port=") {
				p := strings.TrimPrefix(tok, "port=")
				if n, err := strconv.Atoi(p); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func firstAnnotationLine(src string) string {
	// Expect a line like:
	//   // @atomic route=get:users auth=apikey env=.env port=8081   (Go)
	//   # @atomic route=get:users auth=apikey env=.env port=8081    (Python/Node)
	for _, ln := range strings.Split(src, "\n") {
		s := strings.TrimSpace(ln)
		if strings.HasPrefix(s, "// @atomic ") || strings.HasPrefix(s, "# @atomic ") {
			return s
		}
	}
	return ""
}
