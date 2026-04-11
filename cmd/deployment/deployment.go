package deployment

import "github.com/spf13/cobra"

// GetCmd returns the "drift slice" command group for managing slices.
func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deployment",
		Short:   "Manage a drift deployment",
		Example: "  drift deployment run",
		GroupID: "deployment",
	}
	cmd.AddCommand(
		getRunCmd(),
	)
	return cmd
}
