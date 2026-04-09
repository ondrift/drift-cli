package cmd

import (
	slate_cmd "cli/cmd/canvas/cmd"

	"github.com/spf13/cobra"
)

func GetCmd() *cobra.Command {
	slateCmd := &cobra.Command{
		Use:     "canvas",
		Short:   "Manage Canvas hosting",
		GroupID: "services",
	}

	slateCmd.AddCommand(slate_cmd.Deploy())
	slateCmd.GroupID = "services"

	return slateCmd
}
