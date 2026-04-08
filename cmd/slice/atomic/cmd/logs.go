package atomic_cmd

import (
	"encoding/json"
	"fmt"
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
				return common.TransportError("fetch logs", err)
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "fetch logs")
			if err != nil {
				return err
			}

			var entries []struct {
				Timestamp time.Time `json:"timestamp"`
				Line      string    `json:"line"`
			}
			if err := json.Unmarshal(b, &entries); err != nil {
				return fmt.Errorf("Couldn't fetch logs: the API response didn't look right (%w)", err)
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
