package lifecycle

import (
	"bytes"
	"encoding/json"
	"fmt"
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
				fmt.Println(common.TransportError("create slice", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "create slice"); err != nil {
				fmt.Println(err)
				return
			}

			// Auto-set as active slice.
			if err := common.SaveActiveSlice(name); err != nil {
				fmt.Println("Warning: couldn't mark the new slice as active —", err)
			}

			fmt.Printf("Slice '%s' created and set as active.\n", name)
		},
	}

	cmd.Flags().StringVarP(&tier, "tier", "t", "hacker", "Tier for the new slice (hacker, prototyper, etc.)")
	return cmd
}
