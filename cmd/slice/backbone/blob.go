package backbone

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func blobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blob",
		Short: "Store and retrieve binary blobs in your slice",
	}
	cmd.AddCommand(blobPutCmd(), blobGetCmd(), blobListCmd(), blobDeleteCmd())
	return cmd
}

func blobPutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "put <bucket> <key> <file>",
		Short: "Upload a file to a blob bucket",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key, file := args[0], args[1], args[2]

			f, err := os.Open(file)
			if err != nil {
				fmt.Printf("❌ Failed to open file: %v\n", err)
				return
			}
			defer f.Close()

			url := fmt.Sprintf("http://api.localhost:30036/ops/backbone/blob/put?bucket=%s&key=%s", bucket, key)
			req, err := common.NewAuthenticatedRequest(http.MethodPost, url, f)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/octet-stream")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to put blob: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Blob stored at %s/%s\n", bucket, key)
		},
	}
}

func blobGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <bucket> <key>",
		Short: "Download a blob and write it to stdout",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := args[0], args[1]

			url := fmt.Sprintf("http://api.localhost:30036/ops/backbone/blob/get?bucket=%s&key=%s", bucket, key)
			req, err := common.NewAuthenticatedRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Fprintf(os.Stderr, "❌ Failed to get blob: %s\n", string(b))
				os.Exit(1)
				return
			}

			if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error reading blob: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func blobListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <bucket>",
		Short: "List keys in a blob bucket",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bucket := args[0]

			url := fmt.Sprintf("http://api.localhost:30036/ops/backbone/blob/list?bucket=%s", bucket)
			req, err := common.NewAuthenticatedRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to list blobs: %s\n", string(b))
				return
			}

			var keys []string
			if err := json.Unmarshal(b, &keys); err != nil || len(keys) == 0 {
				fmt.Println("No blobs in bucket.")
				return
			}
			for _, k := range keys {
				fmt.Println(k)
			}
		},
	}
}

func blobDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <bucket> <key>",
		Short: "Delete a blob",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bucket, key := args[0], args[1]

			url := fmt.Sprintf("http://api.localhost:30036/ops/backbone/blob/delete?bucket=%s&key=%s", bucket, key)
			req, err := common.NewAuthenticatedRequest(http.MethodPost, url, nil)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to delete blob: %s\n", string(b))
				return
			}

			fmt.Printf("✅ Blob %s/%s deleted\n", bucket, key)
		},
	}
}
