package backbone

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
				fmt.Printf("Couldn't put blob: failed to open %s (%v)\n", file, err)
				return
			}
			defer f.Close()

			url := fmt.Sprintf("%s/ops/backbone/blob/put?bucket=%s&key=%s", common.APIBaseURL, bucket, key)
			resp, err := common.DoRequestWithContentType(http.MethodPost, url, "application/octet-stream", f)
			if err != nil {
				fmt.Println(common.TransportError("put blob", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "put blob"); err != nil {
				fmt.Println(err)
				return
			}

			fmt.Printf("Blob stored at %s/%s\n", bucket, key)
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

			url := fmt.Sprintf("%s/ops/backbone/blob/get?bucket=%s&key=%s", common.APIBaseURL, bucket, key)
			resp, err := common.DoRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, common.TransportError("get blob", err))
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				if _, err := common.CheckResponse(resp, "get blob"); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			}

			if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
				fmt.Fprintf(os.Stderr, "Couldn't get blob: error while streaming the body (%v)\n", err)
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

			url := fmt.Sprintf("%s/ops/backbone/blob/list?bucket=%s", common.APIBaseURL, bucket)
			resp, err := common.DoRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("list blobs", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "list blobs")
			if err != nil {
				fmt.Println(err)
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

			url := fmt.Sprintf("%s/ops/backbone/blob/delete?bucket=%s&key=%s", common.APIBaseURL, bucket, key)
			resp, err := common.DoRequest(http.MethodPost, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("delete blob", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "delete blob"); err != nil {
				fmt.Println(err)
				return
			}

			fmt.Printf("Blob %s/%s deleted\n", bucket, key)
		},
	}
}
