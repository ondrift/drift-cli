package lifecycle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/slice/upgrade",
				bytes.NewBuffer(body),
			)
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
