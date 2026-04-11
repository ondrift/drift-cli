package cmd

import (
	slate_cmd "cli/cmd/canvas/cmd"

	"github.com/spf13/cobra"
)

func GetCmd() *cobra.Command {
	slateCmd := &cobra.Command{
		Use:     "canvas",
		Short:   "Manage Canvas hosting",
		Example: "  drift canvas deploy ./my-site\n  drift canvas deploy ./my-site --site landing-page",
		GroupID: "services",
	}

	slateCmd.AddCommand(slate_cmd.Deploy())
	slateCmd.GroupID = "services"

	return slateCmd
}
