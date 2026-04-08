package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"cli/common"

	"github.com/spf13/cobra"
)

func nosqlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nosql",
		Short: "Read and write JSON documents to the Backbone NoSQL store",
	}
	cmd.AddCommand(nosqlWriteCmd(), nosqlReadCmd(), nosqlListCmd(), nosqlDropCmd())
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
				fmt.Println("Couldn't write document: --data is required.")
				return
			}

			var body map[string]any
			if err := json.Unmarshal([]byte(data), &body); err != nil {
				fmt.Println("Couldn't write document: that doesn't look like valid JSON —", err)
				return
			}
			if collection != "" {
				body["collection"] = collection
			}

			payload, _ := json.Marshal(body)
			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/backbone/write",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println(common.TransportError("write document", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "write document"); err != nil {
				fmt.Println(err)
				return
			}

			if collection != "" {
				fmt.Printf("Document written to collection %q\n", collection)
			} else {
				fmt.Println("Document written")
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

			url := fmt.Sprintf("%s/ops/backbone/nosql/drop?collection=%s", common.APIBaseURL, name)
			resp, err := common.DoRequest(http.MethodPost, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("drop collection", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "drop collection"); err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("Collection %q dropped\n", name)
		},
	}
}

func nosqlListCmd() *cobra.Command {
	var collection, field, value string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all documents in a collection, with optional field filtering",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			url := fmt.Sprintf("%s/ops/backbone/nosql/list?collection=%s&limit=%d", common.APIBaseURL, collection, limit)
			if field != "" && value != "" {
				url += "&field=" + field + "&value=" + value
			}

			resp, err := common.DoRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("list documents", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "list documents")
			if err != nil {
				fmt.Println(err)
				return
			}

			// Pretty-print the JSON array.
			var docs []json.RawMessage
			if err := json.Unmarshal(b, &docs); err != nil {
				fmt.Println(string(b))
				return
			}

			if len(docs) == 0 {
				fmt.Println("(no documents)")
				return
			}

			for _, doc := range docs {
				var pretty bytes.Buffer
				json.Indent(&pretty, doc, "", "  ")
				fmt.Println(pretty.String())
			}
			fmt.Printf("\n%d document(s)\n", len(docs))
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "default", "Collection name")
	cmd.Flags().StringVar(&field, "field", "", "Filter by field name")
	cmd.Flags().StringVar(&value, "value", "", "Filter by field value (requires --field)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of documents to return")
	return cmd
}

func nosqlReadCmd() *cobra.Command {
	var collection, key string
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read a document by key from a collection",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if key == "" {
				fmt.Println("Couldn't read document: --key is required.")
				return
			}

			url := fmt.Sprintf("%s/ops/backbone/read?key=%s", common.APIBaseURL, key)
			if collection != "" {
				url += "&collection=" + collection
			}

			resp, err := common.DoRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("read document", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "read document")
			if err != nil {
				fmt.Println(err)
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
