package lifecycle

import (
	"bytes"
	"encoding/json"
	"fmt"
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
				fmt.Println(common.TransportError("upgrade slice", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "upgrade slice"); err != nil {
				fmt.Println(err)
				return
			}

			fmt.Printf("Slice '%s' upgraded to %s.\n", sliceName, tier)
		},
	}
}
