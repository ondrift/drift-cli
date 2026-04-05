package lifecycle

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func getDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a slice and all its resources (irreversible)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			confirm := common.PromptForInput(fmt.Sprintf("Type '%s' to confirm deletion", name))
			if confirm != name {
				fmt.Println("Deletion cancelled.")
				return
			}

			req, err := common.NewAuthenticatedRequest(
				http.MethodDelete,
				"http://api.localhost:30036/ops/slice/delete?name="+name,
				nil,
			)
			if err != nil {
				fmt.Println("Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 120 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				fmt.Printf("Slice deletion failed: %s\n", string(body))
				return
			}

			// Clear active slice if it was the deleted one.
			if common.GetActiveSlice() == name {
				_ = common.SaveActiveSlice("")
			}

			fmt.Printf("Slice '%s' deleted.\n", name)
		},
	}
}
