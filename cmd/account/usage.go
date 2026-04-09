package account

import (
	"github.com/spf13/cobra"
)

// GetAccountCmd returns the "drift account" command group.
// Subcommands: signup, login.
func GetAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account",
		Short:   "Manage your Drift account",
		GroupID: "account",
	}
	cmd.AddCommand(
		GetCreateCmd(),
		GetLoginCmd(),
	)
	return cmd
}
