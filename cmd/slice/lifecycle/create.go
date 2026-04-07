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

func getCreateCmd() *cobra.Command {
	var tier string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new slice (project)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			if tier == "" {
				tier = "hacker"
			}

			body, _ := json.Marshal(map[string]string{
				"name": name,
				"tier": tier,
			})

			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/slice/create",
				bytes.NewBuffer(body),
			)
			if err != nil {
				fmt.Println("Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			respBytes, _ := io.ReadAll(resp.Body)
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fmt.Printf("Slice creation failed: %s\n", string(respBytes))
				return
			}

			// Auto-set as active slice.
			if err := common.SaveActiveSlice(name); err != nil {
				fmt.Println("Warning: failed to set active slice:", err)
			}

			fmt.Printf("Slice '%s' created and set as active.\n", name)
		},
	}

	cmd.Flags().StringVarP(&tier, "tier", "t", "hacker", "Tier for the new slice (hacker, prototyper, etc.)")
	return cmd
}
