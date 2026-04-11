package deployment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	atomic_cmd "cli/cmd/atomic/cmd/deploy"
	atomic_common "cli/cmd/atomic/common"
	"cli/common"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Manifest is the top-level structure of a drift.yaml deployment file.
type Manifest struct {
	Name     string        `yaml:"name"`
	Canvas   []CanvasEntry `yaml:"canvas"`
	Atomic   []AtomicEntry `yaml:"atomic"`
	Backbone BackboneSpec  `yaml:"backbone"`
}

type CanvasEntry struct {
	Dir  string `yaml:"dir"`
	Site string `yaml:"site"` // defaults to "default"
}

type AtomicEntry struct {
	Dir     string `yaml:"dir"`
	Element string `yaml:"element"` // optional grouping element
}

type BackboneSpec struct {
	Cache   []CacheEntry  `yaml:"cache"`
	NoSQL   []NoSQLEntry  `yaml:"nosql"`
	Queues  []QueueEntry  `yaml:"queues"`
	Secrets []SecretEntry `yaml:"secrets"`
}

// CacheEntry seeds a Backbone cache key. Provide either file (path to a JSON
// file whose contents become the value) or value (inline string).
type CacheEntry struct {
	Key   string `yaml:"key"`
	File  string `yaml:"file"`
	Value string `yaml:"value"`
	TTL   int    `yaml:"ttl"` // seconds; 0 = no expiry
}

// NoSQLEntry primes a Backbone NoSQL collection by writing a sentinel document.
type NoSQLEntry struct {
	Collection string `yaml:"collection"`
}

// QueueEntry primes a Backbone queue by pushing a sentinel message.
type QueueEntry struct {
	Name string `yaml:"name"`
}

// SecretEntry stores a Backbone secret. Use env to read the value from a
// local environment variable (never written to the manifest file) or value
// for non-sensitive config.
type SecretEntry struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
	Env   string `yaml:"env"`
}

func getRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Deploy all resources declared in a drift.yaml manifest",
		Example: "  drift deployment run",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := common.RequireActiveSlice(); err != nil {
				return err
			}

			manifestPath, err := filepath.Abs(filepath.Join(".", "drift.yaml"))
			if err != nil {
				return fmt.Errorf("failed to resolve manifest path: %w", err)
			}

			data, err := os.ReadFile(manifestPath) // #nosec G304 — CLI reads user's manifest by design
			if err != nil {
				return fmt.Errorf("failed to read manifest: %w", err)
			}

			var m Manifest
			if err := yaml.Unmarshal(data, &m); err != nil {
				return fmt.Errorf("failed to parse manifest: %w", err)
			}

			baseDir := filepath.Dir(manifestPath)
			start := time.Now()

			fmt.Printf("\n  Deploying %s...\n\n", common.Highlight(m.Name))

			// ── Atomic ──────────────────────────────────────────────────────
			if len(m.Atomic) > 0 {
				fmt.Printf("  %s\n", common.AtomicHeader())
				for _, entry := range m.Atomic {
					dir := resolve(baseDir, entry.Dir)
					// Resolve the canonical function name up-front so the
					// spinner can show it before the build starts.
					_, fnName, _, metaErr := atomic_common.ParseAtomicMetadataFromDir(dir)
					if metaErr != nil || fnName == "" {
						fnName = filepath.Base(entry.Dir)
					}
					sp := common.StartSpinner("    ", fnName)
					err := atomic_cmd.DeployFolder(dir, entry.Element, true)
					sp.Stop()
					if err != nil {
						return fmt.Errorf("atomic deploy failed for %s: %w", entry.Dir, err)
					}
					fmt.Printf("    %s %s\n", common.Check(), fnName)
				}
				fmt.Println()
			}

			// ── Backbone ────────────────────────────────────────────────────
			bb := m.Backbone
			hasBackbone := len(bb.Cache)+len(bb.NoSQL)+len(bb.Queues)+len(bb.Secrets) > 0
			if hasBackbone {
				fmt.Printf("  %s\n", common.BackboneHeader())

				for _, entry := range bb.Cache {
					label := fmt.Sprintf("Cache: %s", entry.Key)
					sp := common.StartSpinner("    ", label)
					var value string
					if entry.File != "" {
						raw, err := os.ReadFile(resolve(baseDir, entry.File)) // #nosec G304
						if err != nil {
							sp.Stop()
							return fmt.Errorf("failed to read cache file %s: %w", entry.File, err)
						}
						value = string(raw)
					} else {
						value = entry.Value
					}
					if err := cacheSet(entry.Key, value, entry.TTL); err != nil {
						sp.Stop()
						return fmt.Errorf("cache set %q failed: %w", entry.Key, err)
					}
					sp.Stop()
					line := fmt.Sprintf("    %s %s", common.Check(), label)
					if entry.File != "" {
						line += " " + common.Hint(fmt.Sprintf("(seeded from %s)", filepath.Base(entry.File)))
					}
					fmt.Println(line)
				}

				for _, entry := range bb.NoSQL {
					label := fmt.Sprintf("NoSQL: %s", entry.Collection)
					sp := common.StartSpinner("    ", label)
					if err := nosqlInit(entry.Collection); err != nil {
						sp.Stop()
						return fmt.Errorf("nosql init %q failed: %w", entry.Collection, err)
					}
					sp.Stop()
					fmt.Printf("    %s %s\n", common.Check(), label)
				}

				for _, entry := range bb.Queues {
					label := fmt.Sprintf("Queue: %s", entry.Name)
					sp := common.StartSpinner("    ", label)
					if err := queueInit(entry.Name); err != nil {
						sp.Stop()
						return fmt.Errorf("queue init %q failed: %w", entry.Name, err)
					}
					sp.Stop()
					fmt.Printf("    %s %s\n", common.Check(), label)
				}

				if len(bb.Secrets) > 0 {
					sp := common.StartSpinner("    ", "Secrets: injecting…")
					injected := 0
					var skipped []string
					for _, entry := range bb.Secrets {
						value := entry.Value
						if entry.Env != "" {
							value = os.Getenv(entry.Env)
							if value == "" {
								skipped = append(skipped, fmt.Sprintf("%s (env %s unset)", entry.Key, entry.Env))
								continue
							}
						}
						if err := secretSet(entry.Key, value); err != nil {
							sp.Stop()
							return fmt.Errorf("secret set %q failed: %w", entry.Key, err)
						}
						injected++
					}
					sp.Stop()
					if injected > 0 {
						fmt.Printf("    %s Secrets: %d injected\n", common.Check(), injected)
					}
					for _, msg := range skipped {
						fmt.Printf("    %s Secret %s\n", common.Hint("!"), msg)
					}
				}

				fmt.Println()
			}

			// ── Canvas ──────────────────────────────────────────────────────
			if len(m.Canvas) > 0 {
				fmt.Printf("  %s\n", common.CanvasHeader())
				for _, entry := range m.Canvas {
					dir := resolve(baseDir, entry.Dir)
					site := entry.Site
					if site == "" {
						site = "default"
					}
					sp := common.StartSpinner("    ", site)
					if err := deployCanvas(dir, site); err != nil {
						sp.Stop()
						return fmt.Errorf("canvas deploy failed for %s: %w", entry.Dir, err)
					}
					sp.Stop()
					fmt.Printf("    %s %s\n", common.Check(), site)
				}
				fmt.Println()
			}

			elapsed := time.Since(start).Seconds()
			fmt.Printf("  %s\n\n", common.Hint(fmt.Sprintf("Done in %.1fs!", elapsed)))
			return nil
		},
	}
}

// resolve returns an absolute path by joining base with rel when rel is relative.
func resolve(base, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(base, rel)
}

func deployCanvas(dir, site string) error {
	zipData, err := common.ZipFolder(dir)
	if err != nil {
		return fmt.Errorf("failed to zip folder: %w", err)
	}
	resp, err := common.DoRequestWithHeaders(
		http.MethodPost,
		common.APIBaseURL+"/ops/canvas",
		zipData,
		map[string]string{
			"Content-Type":  "application/zip",
			"X-Canvas-Site": site,
		},
	)
	if err != nil {
		return common.TransportError("deploy canvas site", err)
	}
	defer resp.Body.Close()
	_, err = common.CheckResponse(resp, "deploy canvas site")
	return err
}

func cacheSet(key, value string, ttl int) error {
	payload, _ := json.Marshal(map[string]any{"key": key, "value": value, "ttl": ttl})
	resp, err := common.DoJSONRequest(http.MethodPost, common.APIBaseURL+"/ops/backbone/cache/set", bytes.NewBuffer(payload))
	if err != nil {
		return common.TransportError("seed cache key", err)
	}
	defer resp.Body.Close()
	_, err = common.CheckResponse(resp, "seed cache key")
	return err
}

func nosqlInit(collection string) error {
	payload, _ := json.Marshal(map[string]any{"collection": collection, "_setup": true})
	resp, err := common.DoJSONRequest(http.MethodPost, common.APIBaseURL+"/ops/backbone/write", bytes.NewBuffer(payload))
	if err != nil {
		return common.TransportError("initialise NoSQL collection", err)
	}
	defer resp.Body.Close()
	_, err = common.CheckResponse(resp, "initialise NoSQL collection")
	return err
}

func queueInit(name string) error {
	payload, _ := json.Marshal(map[string]any{"queue": name, "body": map[string]any{"_setup": true}})
	resp, err := common.DoJSONRequest(http.MethodPost, common.APIBaseURL+"/ops/backbone/queue/push", bytes.NewBuffer(payload))
	if err != nil {
		return common.TransportError("initialise queue", err)
	}
	defer resp.Body.Close()
	_, err = common.CheckResponse(resp, "initialise queue")
	return err
}

func secretSet(name, value string) error {
	payload, _ := json.Marshal(map[string]string{"name": name, "value": value})
	resp, err := common.DoJSONRequest(http.MethodPost, common.APIBaseURL+"/ops/backbone/secret/set", bytes.NewBuffer(payload))
	if err != nil {
		return common.TransportError("store secret", err)
	}
	defer resp.Body.Close()
	_, err = common.CheckResponse(resp, "store secret")
	return err
}
