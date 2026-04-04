package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func nosqlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nosql",
		Short: "Read and write JSON documents to the Backbone NoSQL store",
	}
	cmd.AddCommand(nosqlWriteCmd(), nosqlReadCmd(), nosqlDropCmd())
	return cmd
}

func nosqlWriteCmd() *cobra.Command {
	var collection, data string
	cmd := &cobra.Command{
		Use:   "write",
		Short: "Write a JSON document to a collection",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if data == "" {
				fmt.Println("❌ --data is required")
				return
			}

			var body map[string]any
			if err := json.Unmarshal([]byte(data), &body); err != nil {
				fmt.Println("❌ Invalid JSON:", err)
				return
			}
			if collection != "" {
				body["collection"] = collection
			}

			payload, _ := json.Marshal(body)
			req, err := common.NewAuthenticatedRequest(
				http.MethodPost,
				"http://api.localhost:30036/ops/backbone/write",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Not logged in:", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Write failed: %s\n", string(b))
				return
			}

			if collection != "" {
				fmt.Printf("✅ Document written to collection %q\n", collection)
			} else {
				fmt.Println("✅ Document written")
			}
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "Collection name (default: \"default\")")
	cmd.Flags().StringVar(&data, "data", "", "JSON document to write")
	_ = cmd.MarkFlagRequired("data")
	return cmd
}

func nosqlDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop <collection>",
		Short: "Delete a NoSQL collection and all its documents",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			url := fmt.Sprintf("http://api.localhost:30036/ops/backbone/nosql/drop?collection=%s", name)
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

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to drop collection: %s\n", string(b))
				return
			}
			fmt.Printf("✅ Collection %q dropped\n", name)
		},
	}
}

func nosqlReadCmd() *cobra.Command {
	var collection, key string
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read a document by key from a collection",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if key == "" {
				fmt.Println("❌ --key is required")
				return
			}

			url := fmt.Sprintf("http://api.localhost:30036/ops/backbone/read?key=%s", key)
			if collection != "" {
				url += "&collection=" + collection
			}

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
				fmt.Printf("❌ Read failed: %s\n", string(b))
				return
			}

			fmt.Println(string(b))
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "", "Collection name (default: \"default\")")
	cmd.Flags().StringVar(&key, "key", "", "Document key to retrieve")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}
