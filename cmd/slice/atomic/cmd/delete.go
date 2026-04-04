package atomic_cmd

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func Delete() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a deployed atomic function (e.g. atomic-1)",
		GroupID: "operations",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]

			req, err := common.NewAuthenticatedRequest(
				http.MethodDelete,
				"http://api.localhost:30036/ops/atomic/delete?id="+id,
				nil,
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to delete function: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Function %q deleted\n", id)
		},
	}
}
