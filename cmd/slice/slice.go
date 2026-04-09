package slice

import "github.com/spf13/cobra"

// GetCmd returns the "drift slice" command group for managing slices.
func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "slice",
		Short:   "Manage your Drift slices (projects)",
		GroupID: "account",
	}
	cmd.AddCommand(
		getCreateCmd(),
		getResizeCmd(),
		getListCmd(),
		getUseCmd(),
		getDeleteCmd(),
		getPlanCmd(),
	)
	return cmd
}
