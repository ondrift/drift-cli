package atomic

import (
	"github.com/spf13/cobra"

	cmd "cli/cmd/atomic/cmd"
	cmd_deploy "cli/cmd/atomic/cmd/deploy"
	cmd_new "cli/cmd/atomic/cmd/new"
	cmd_run "cli/cmd/atomic/cmd/run"
)

func GetCmd() *cobra.Command {
	atomicCmd := &cobra.Command{
		Use:     "atomic",
		Short:   "Manage Atomic deployments",
		Example: "  drift atomic list\n  drift atomic deploy ./my-function\n  drift atomic run ./my-function\n  drift atomic new",
		GroupID: "services",
	}

	atomicCmd.AddGroup(&cobra.Group{
		ID:    "development",
		Title: "Development",
	})

	atomicCmd.AddGroup(&cobra.Group{
		ID:    "operations",
		Title: "Operations",
	})

	atomicCmd.AddCommand(
		cmd_deploy.Deploy(),
		cmd_run.Run(),
		cmd.Auth(),
		cmd.Delete(),
		cmd.Element(),
		cmd.List(),
		cmd.Logs(),
		cmd.Metrics(),
		cmd.Trigger(),
		cmd_new.Go(),
	)

	atomicCmd.GroupID = "services"

	return atomicCmd
}
