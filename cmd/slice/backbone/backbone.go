package backbone

import "github.com/spf13/cobra"

// GetCmd returns the "backbone" command group, which exposes the Drift
// backbone primitives (secrets, queues, blobs) directly from the CLI.
func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "backbone",
		Short:   "Interact with your slice's backbone primitives",
		GroupID: "services",
	}

	cmd.AddCommand(secretCmd(), blobCmd(), queueCmd(), lockCmd(), nosqlCmd(), cacheCmd())
	return cmd
}
