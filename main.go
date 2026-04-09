package main

import (
	_ "embed"
	"os"

	account "cli/cmd/account"
	atomic "cli/cmd/atomic"
	backbone "cli/cmd/backbone"
	canvas "cli/cmd/canvas"
	deployment "cli/cmd/deployment"
	slice "cli/cmd/slice"

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

	rootCmd.AddGroup(&cobra.Group{
		ID:    "deployment",
		Title: "Deployment:",
	})

	rootCmd.AddCommand(
		// Declarative deployment
		deployment.GetCmd(),

		// Deployment planning
		account.GetPlanCmd(),

		// Atomic functions
		atomic.GetCmd(),

		// Canvas (static sites)
		canvas.GetCmd(),

		// Backbone primitives (secrets, queues, blobs)
		backbone.GetCmd(),

		// Slice lifecycle (create, list, use, delete, upgrade)
		slice.GetCmd(),

		// Account (signup, login, usage, upgrade)
		account.GetAccountCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
