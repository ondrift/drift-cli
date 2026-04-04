package account

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

// GetAccountCmd returns the "drift account" command group.
// Subcommands: usage, signup, login, upgrade.
func GetAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account",
		Short:   "Manage your Drift account",
		GroupID: "account",
	}
	cmd.AddCommand(
		getUsageCmd(),
		GetSignupCmd(),
		GetLoginCmd(),
		getUpgradeCmd(),
	)
	return cmd
}

func getUsageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Show your current plan and resource usage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := fetchUsage()
			if err != nil {
				return err
			}
			printUsage(u)
			return nil
		},
	}
}

func getUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade <tier>",
		Short: "Upgrade your plan to a higher tier (e.g. prototyper)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tier := args[0]

			body, err := json.Marshal(map[string]string{"tier": tier})
			if err != nil {
				fmt.Println("❌ Failed to build request:", err)
				return
			}

			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/plan/upgrade",
				bytes.NewBuffer(body),
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			respBytes, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Upgrade failed: %s\n", string(respBytes))
				return
			}

			fmt.Printf("✅ Plan upgraded to %s\n", tier)
		},
	}
}
