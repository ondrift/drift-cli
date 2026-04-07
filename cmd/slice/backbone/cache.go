package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"cli/common"

	"github.com/spf13/cobra"
)

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "In-memory key-value cache in your slice",
	}
	cmd.AddCommand(cacheSetCmd(), cacheGetCmd(), cacheDelCmd(), cacheExistsCmd())
	return cmd
}

func cacheSetCmd() *cobra.Command {
	var ttl int
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a cache key",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key, value := args[0], args[1]

			payload, _ := json.Marshal(map[string]any{"key": key, "value": value, "ttl": ttl})
			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/backbone/cache/set",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to set key: %s\n", string(b))
				return
			}

			if ttl > 0 {
				fmt.Printf("✅ %q set (ttl: %ds)\n", key, ttl)
			} else {
				fmt.Printf("✅ %q set (no expiry)\n", key)
			}
		},
	}
	cmd.Flags().IntVar(&ttl, "ttl", 0, "TTL in seconds (0 = no expiry)")
	return cmd
}

func cacheGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a cache value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			resp, err := common.DoRequest(
				http.MethodGet,
				common.APIBaseURL+"/ops/backbone/cache/get?key="+key,
				nil,
			)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("❌ Key %q not found\n", key)
				return
			}
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Error: %s\n", string(b))
				return
			}

			b, _ := io.ReadAll(resp.Body)
			fmt.Println(string(b))
		},
	}
}

func cacheDelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "del <key>",
		Short: "Delete a cache key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			payload, _ := json.Marshal(map[string]string{"key": key})
			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/backbone/cache/del",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("❌ Key %q not found\n", key)
				return
			}
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to delete key: %s\n", string(b))
				return
			}

			fmt.Printf("✅ %q deleted\n", key)
		},
	}
}

func cacheExistsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exists <key>",
		Short: "Check whether a cache key exists",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			resp, err := common.DoRequest(
				http.MethodGet,
				common.APIBaseURL+"/ops/backbone/cache/exists?key="+key,
				nil,
			)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			var result struct {
				Exists bool `json:"exists"`
			}
			if err := json.Unmarshal(b, &result); err != nil {
				fmt.Println("❌ Unexpected response:", string(b))
				return
			}

			if result.Exists {
				fmt.Printf("✅ %q exists\n", key)
			} else {
				fmt.Printf("   %q does not exist\n", key)
			}
		},
	}
}
