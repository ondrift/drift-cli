package lifecycle

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
		getListCmd(),
		getUseCmd(),
		getDeleteCmd(),
		getUpgradeCmd(),
	)
	return cmd
}
