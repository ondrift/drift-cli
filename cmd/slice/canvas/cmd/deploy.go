package slate_cmd

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func Deploy() *cobra.Command {
	var site string

	deployCmd := &cobra.Command{
		Use:   "deploy [directory]",
		Short: "Deploy a static site from a directory",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			folder := args[0]
			fmt.Printf("Deploying canvas site from folder: %s\n", folder)

			zipData, err := common.ZipFolder(folder)
			if err != nil {
				fmt.Printf("failed to zip folder: %v\n", err)
				return
			}

			req, err := common.NewAuthenticatedRequest("POST", "http://api.localhost:30036/ops/canvas", zipData)
			if err != nil {
				fmt.Printf("failed to create upload request: %v\n", err)
				return
			}
			req.Header.Set("Content-Type", "application/zip")
			req.Header.Set("X-Canvas-Site", site)

			client := http.Client{
				Timeout: 30 * time.Second,
			}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("upload failed: %v\n", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				fmt.Printf("upload failed with status %d: %s\n", resp.StatusCode, string(body))
			}

			fmt.Println("Upload successful!")
		},
	}

	deployCmd.Flags().StringVarP(&site, "site", "s", "default", "Canvas site name (accessible at <site>.<username>.ondrift.eu)")

	return deployCmd
}
