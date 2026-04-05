package lifecycle

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

type sliceEntry struct {
	Name string `json:"name"`
	Tier string `json:"tier"`
}

func getListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all your slices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := common.NewAuthenticatedRequest(
				http.MethodGet,
				"http://api.localhost:30036/ops/slice/list",
				nil,
			)
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to contact API: %w", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("failed to list slices: %s", string(body))
			}

			var slices []sliceEntry
			if err := json.Unmarshal(body, &slices); err != nil {
				return fmt.Errorf("invalid response: %w", err)
			}

			active := common.GetActiveSlice()
			for _, s := range slices {
				marker := "  "
				if s.Name == active {
					marker = "* "
				}
				fmt.Printf("%s%-20s %s\n", marker, s.Name, s.Tier)
			}
			return nil
		},
	}
}
