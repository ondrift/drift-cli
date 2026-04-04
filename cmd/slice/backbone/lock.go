package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func lockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Acquire and release distributed locks in your slice",
	}
	cmd.AddCommand(lockAcquireCmd(), lockReleaseCmd(), lockRenewCmd())
	return cmd
}

func lockAcquireCmd() *cobra.Command {
	var ttl int
	cmd := &cobra.Command{
		Use:   "acquire <name> <owner>",
		Short: "Acquire a named lock",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			name, owner := args[0], args[1]

			payload, _ := json.Marshal(map[string]any{"name": name, "owner": owner, "ttl": ttl})
			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/backbone/lock/acquire",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusConflict {
				fmt.Printf("❌ Lock %q is already held\n", name)
				return
			}
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to acquire lock: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Lock %q acquired by %q (ttl: %ds)\n", name, owner, ttl)
		},
	}
	cmd.Flags().IntVar(&ttl, "ttl", 30, "Lock TTL in seconds")
	return cmd
}

func lockReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release <name> <owner>",
		Short: "Release a named lock",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			name, owner := args[0], args[1]

			payload, _ := json.Marshal(map[string]any{"name": name, "owner": owner})
			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/backbone/lock/release",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to release lock: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Lock %q released\n", name)
		},
	}
}

func lockRenewCmd() *cobra.Command {
	var ttl int
	cmd := &cobra.Command{
		Use:   "renew <name> <owner>",
		Short: "Renew a lock's TTL",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			name, owner := args[0], args[1]

			payload, _ := json.Marshal(map[string]any{"name": name, "owner": owner, "ttl": ttl})
			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/backbone/lock/renew",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to renew lock: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Lock %q renewed (ttl: %ds)\n", name, ttl)
		},
	}
	cmd.Flags().IntVar(&ttl, "ttl", 30, "New TTL in seconds")
	return cmd
}
