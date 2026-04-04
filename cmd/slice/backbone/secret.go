package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func secretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage encrypted secrets in your slice",
	}
	cmd.AddCommand(secretSetCmd(), secretGetCmd(), secretListCmd(), secretDeleteCmd())
	return cmd
}

func secretSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY=VALUE",
		Short: "Store an encrypted secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			parts := strings.SplitN(args[0], "=", 2)
			if len(parts) != 2 || parts[0] == "" {
				fmt.Println("❌ argument must be in KEY=VALUE format")
				return
			}
			name, value := parts[0], parts[1]

			body, _ := json.Marshal(map[string]string{"name": name, "value": value})
			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/backbone/secret/set",
				bytes.NewBuffer(body),
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

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to store secret: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Secret %q stored\n", name)
		},
	}
}

func secretGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Retrieve the value of a secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			req, err := common.NewAuthenticatedRequest(
				http.MethodGet,
				"http://api.localhost:30036/ops/backbone/secret/get?name="+args[0],
				nil,
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to get secret: %s\n", string(b))
				return
			}

			fmt.Println(string(b))
		},
	}
}

func secretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List secret names",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			req, err := common.NewAuthenticatedRequest(
				http.MethodGet,
				"http://api.localhost:30036/ops/backbone/secret/list",
				nil,
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to list secrets: %s\n", string(b))
				return
			}

			var names []string
			if err := json.Unmarshal(b, &names); err != nil || len(names) == 0 {
				fmt.Println("No secrets stored.")
				return
			}
			for _, n := range names {
				fmt.Println(n)
			}
		},
	}
}

func secretDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete KEY",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			body, _ := json.Marshal(map[string]string{"name": args[0]})
			req, err := common.NewAuthenticatedRequest(
				http.MethodDelete,
				"http://api.localhost:30036/ops/backbone/secret/delete",
				bytes.NewBuffer(body),
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

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to delete secret: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Secret %q deleted\n", args[0])
		},
	}
}
