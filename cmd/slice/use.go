package slice

import (
	"fmt"

	"cli/common"

	"github.com/spf13/cobra"
)

func getUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active slice for subsequent commands",
		Example: "  drift slice use my-slice",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			if err := common.SaveActiveSlice(name); err != nil {
				fmt.Println("Failed to set active slice:", err)
				return
			}
			fmt.Printf("Active slice set to '%s'.\n", name)
		},
	}
}
