package deploy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	atomic_cmd "cli/cmd/slice/atomic/cmd/deploy"
	"cli/common"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Manifest is the top-level structure of a drift.yaml deployment file.
type Manifest struct {
	Name     string       `yaml:"name"`
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

func GetDeployCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "deploy [drift.yaml]",
		Short:   "Deploy all resources declared in a drift.yaml manifest",
		GroupID: "services",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath, err := filepath.Abs(args[0])
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
			client := &http.Client{Timeout: 30 * time.Second}

			fmt.Printf("🚀 Deploying \"%s\"\n\n", m.Name)

			// ── Canvas ──────────────────────────────────────────────────────
			if len(m.Canvas) > 0 {
				fmt.Println("📦 Canvas")
				for _, entry := range m.Canvas {
					dir := resolve(baseDir, entry.Dir)
					site := entry.Site
					if site == "" {
						site = "default"
					}
					fmt.Printf("  → site %q from %s\n", site, entry.Dir)
					if err := deployCanvas(client, dir, site); err != nil {
						return fmt.Errorf("canvas deploy failed for %s: %w", entry.Dir, err)
					}
					fmt.Printf("  ✅ site %q deployed\n", site)
				}
				fmt.Println()
			}

			// ── Atomic ──────────────────────────────────────────────────────
			if len(m.Atomic) > 0 {
				fmt.Println("⚡ Atomic")
				for _, entry := range m.Atomic {
					dir := resolve(baseDir, entry.Dir)
					if err := atomic_cmd.DeployFolder(dir, entry.Element); err != nil {
						return fmt.Errorf("atomic deploy failed for %s: %w", entry.Dir, err)
					}
				}
				fmt.Println()
			}

			// ── Backbone ────────────────────────────────────────────────────
			bb := m.Backbone
			hasBackbone := len(bb.Cache)+len(bb.NoSQL)+len(bb.Queues)+len(bb.Secrets) > 0
			if hasBackbone {
				fmt.Println("🔧 Backbone")

				for _, entry := range bb.Cache {
					var value string
					if entry.File != "" {
						raw, err := os.ReadFile(resolve(baseDir, entry.File)) // #nosec G304
						if err != nil {
							return fmt.Errorf("failed to read cache file %s: %w", entry.File, err)
						}
						value = string(raw)
					} else {
						value = entry.Value
					}
					if err := cacheSet(client, entry.Key, value, entry.TTL); err != nil {
						return fmt.Errorf("cache set %q failed: %w", entry.Key, err)
					}
					fmt.Printf("  ✅ cache %q seeded\n", entry.Key)
				}

				for _, entry := range bb.NoSQL {
					if err := nosqlInit(client, entry.Collection); err != nil {
						return fmt.Errorf("nosql init %q failed: %w", entry.Collection, err)
					}
					fmt.Printf("  ✅ collection %q ready\n", entry.Collection)
				}

				for _, entry := range bb.Queues {
					if err := queueInit(client, entry.Name); err != nil {
						return fmt.Errorf("queue init %q failed: %w", entry.Name, err)
					}
					fmt.Printf("  ✅ queue %q ready\n", entry.Name)
				}

				for _, entry := range bb.Secrets {
					value := entry.Value
					if entry.Env != "" {
						value = os.Getenv(entry.Env)
						if value == "" {
							fmt.Printf("  ⚠️  secret %q: env var %s is not set, skipping\n", entry.Key, entry.Env)
							continue
						}
					}
					if err := secretSet(client, entry.Key, value); err != nil {
						return fmt.Errorf("secret set %q failed: %w", entry.Key, err)
					}
					fmt.Printf("  ✅ secret %q stored\n", entry.Key)
				}

				fmt.Println()
			}

			fmt.Printf("✅ \"%s\" deployed successfully!\n", m.Name)
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

func deployCanvas(client *http.Client, dir, site string) error {
	zipData, err := common.ZipFolder(dir)
	if err != nil {
		return fmt.Errorf("failed to zip folder: %w", err)
	}
	req, err := common.NewAuthenticatedRequest("POST", "http://api.localhost:30036/ops/canvas", zipData)
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("X-Canvas-Site", site)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func cacheSet(client *http.Client, key, value string, ttl int) error {
	payload, _ := json.Marshal(map[string]any{"key": key, "value": value, "ttl": ttl})
	req, err := common.NewAuthenticatedRequest(http.MethodPost, "http://api.localhost:30036/ops/backbone/cache/set", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func nosqlInit(client *http.Client, collection string) error {
	payload, _ := json.Marshal(map[string]any{"collection": collection, "_setup": true})
	req, err := common.NewAuthenticatedRequest(http.MethodPost, "http://api.localhost:30036/ops/backbone/write", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func queueInit(client *http.Client, name string) error {
	payload, _ := json.Marshal(map[string]any{"queue": name, "body": map[string]any{"_setup": true}})
	req, err := common.NewAuthenticatedRequest(http.MethodPost, "http://api.localhost:30036/ops/backbone/queue/push", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func secretSet(client *http.Client, name, value string) error {
	payload, _ := json.Marshal(map[string]string{"name": name, "value": value})
	req, err := common.NewAuthenticatedRequest(http.MethodPost, "http://api.localhost:30036/ops/backbone/secret/set", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("not logged in: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
