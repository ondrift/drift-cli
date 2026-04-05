package lifecycle

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

func getUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade <tier>",
		Short: "Change the tier of the active slice",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tier := args[0]

			sliceName, err := common.RequireActiveSlice()
			if err != nil {
				fmt.Println(err)
				return
			}

			body, _ := json.Marshal(map[string]string{
				"name": sliceName,
				"tier": tier,
			})

			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/slice/upgrade",
				bytes.NewBuffer(body),
			)
			if err != nil {
				fmt.Println("Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			respBytes, _ := io.ReadAll(resp.Body)
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fmt.Printf("Upgrade failed: %s\n", string(respBytes))
				return
			}

			fmt.Printf("Slice '%s' upgraded to %s.\n", sliceName, tier)
		},
	}
}
