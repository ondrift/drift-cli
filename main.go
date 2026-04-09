package main

import (
	_ "embed"
	"os"

	account "cli/cmd/account"
	deploy "cli/cmd/deploy"
	atomic "cli/cmd/slice/atomic"
	backbone "cli/cmd/slice/backbone"
	canvas "cli/cmd/slice/canvas"
	lifecycle "cli/cmd/slice/lifecycle"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "drift",
		Short:         "Drift is a minimalist cloud hosting service.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.AddGroup(&cobra.Group{
		ID:    "services",
		Title: "Services:",
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    "account",
		Title: "Account:",
	})

	rootCmd.AddCommand(
		// Declarative deployment
		deploy.GetDeployCmd(),

		// Deployment planning
		account.GetPlanCmd(),

		// Atomic functions
		atomic.GetCmd(),

		// Canvas (static sites)
		canvas.GetCmd(),

		// Backbone primitives (secrets, queues, blobs)
		backbone.GetCmd(),

		// Slice lifecycle (create, list, use, delete, upgrade)
		lifecycle.GetCmd(),

		// Account (signup, login, usage, upgrade)
		account.GetAccountCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
