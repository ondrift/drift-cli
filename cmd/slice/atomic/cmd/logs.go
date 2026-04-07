package atomic_cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func Logs() *cobra.Command {
	var tail int

	cmd := &cobra.Command{
		Use:     "logs <function-name>",
		Short:   "Fetch recent logs for a deployed function",
		GroupID: "operations",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			function := args[0]

			resp, err := common.DoRequest(
				http.MethodGet,
				common.APIBaseURL+"/ops/atomic/logs?function="+function,
				nil,
			)
			if err != nil {
				return fmt.Errorf("could not reach API: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(b))
			}

			var entries []struct {
				Timestamp time.Time `json:"timestamp"`
				Line      string    `json:"line"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if len(entries) == 0 {
				fmt.Printf("No logs found for %q\n", function)
				return nil
			}

			if tail > 0 && tail < len(entries) {
				entries = entries[len(entries)-tail:]
			}

			for _, e := range entries {
				fmt.Printf("%s  %s\n", e.Timestamp.Format("2006-01-02 15:04:05.000"), e.Line)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&tail, "tail", "n", 0, "Show only the N most recent lines (default: all)")
	return cmd
}
