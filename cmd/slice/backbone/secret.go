package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"cli/common"

	"github.com/spf13/cobra"
)

func secretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage encrypted secrets in your slice",
	}
	cmd.AddCommand(secretSetCmd(), secretGetCmd(), secretListCmd(), secretDeleteCmd())
	return cmd
}

func secretSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY=VALUE",
		Short: "Store an encrypted secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			parts := strings.SplitN(args[0], "=", 2)
			if len(parts) != 2 || parts[0] == "" {
				fmt.Println("Couldn't store secret: argument must be in KEY=VALUE format.")
				return
			}
			name, value := parts[0], parts[1]

			body, _ := json.Marshal(map[string]string{"name": name, "value": value})
			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/backbone/secret/set",
				bytes.NewBuffer(body),
			)
			if err != nil {
				fmt.Println(common.TransportError("store secret", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "store secret"); err != nil {
				fmt.Println(err)
				return
			}

			fmt.Printf("Secret %q stored\n", name)
		},
	}
}

func secretGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get KEY",
		Short: "Retrieve the value of a secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := common.DoRequest(
				http.MethodGet,
				common.APIBaseURL+"/ops/backbone/secret/get?name="+args[0],
				nil,
			)
			if err != nil {
				fmt.Println(common.TransportError("get secret", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "get secret")
			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Println(string(b))
		},
	}
}

func secretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List secret names",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := common.DoRequest(
				http.MethodGet,
				common.APIBaseURL+"/ops/backbone/secret/list",
				nil,
			)
			if err != nil {
				fmt.Println(common.TransportError("list secrets", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "list secrets")
			if err != nil {
				fmt.Println(err)
				return
			}

			var names []string
			if err := json.Unmarshal(b, &names); err != nil || len(names) == 0 {
				fmt.Println("No secrets stored.")
				return
			}
			for _, n := range names {
				fmt.Println(n)
			}
		},
	}
}

func secretDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete KEY",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			body, _ := json.Marshal(map[string]string{"name": args[0]})
			resp, err := common.DoJSONRequest(
				http.MethodDelete,
				common.APIBaseURL+"/ops/backbone/secret/delete",
				bytes.NewBuffer(body),
			)
			if err != nil {
				fmt.Println(common.TransportError("delete secret", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "delete secret"); err != nil {
				fmt.Println(err)
				return
			}

			fmt.Printf("Secret %q deleted\n", args[0])
		},
	}
}
