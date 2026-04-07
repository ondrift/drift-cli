package atomic_cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cli/common"

	"github.com/spf13/cobra"
)

func Auth() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth",
		Short:   "Manage API key authentication for deployed functions",
		GroupID: "operations",
	}
	cmd.AddCommand(authSet(), authList(), authRevoke())
	return cmd
}

// authSet sets or rotates the API key for a function+method.
func authSet() *cobra.Command {
	var method string

	cmd := &cobra.Command{
		Use:   "set <function-name> <api-key>",
		Short: "Set or rotate the API key for a deployed function",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			function := args[0]
			key := args[1]

			body, _ := json.Marshal(map[string]string{
				"function": function,
				"method":   strings.ToUpper(method),
				"key":      key,
			})

			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/atomic/auth",
				bytes.NewReader(body),
			)
			if err != nil {
				return fmt.Errorf("could not reach API: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(b))
			}

			fmt.Printf("API key set for %s %s\n", strings.ToUpper(method), function)
			return nil
		},
	}

	cmd.Flags().StringVarP(&method, "method", "m", "post", "HTTP method of the function (get, post, put, delete)")
	return cmd
}

// authList shows key fingerprints configured for a function.
func authList() *cobra.Command {
	return &cobra.Command{
		Use:   "list <function-name>",
		Short: "List API keys configured for a deployed function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			function := args[0]

			resp, err := common.DoRequest(
				http.MethodGet,
				common.APIBaseURL+"/ops/atomic/auth?function="+function,
				nil,
			)
			if err != nil {
				return fmt.Errorf("could not reach API: %w", err)
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(b))
			}

			var keys []struct {
				Method      string `json:"method"`
				Path        string `json:"path"`
				Fingerprint string `json:"fingerprint"`
			}
			if err := json.Unmarshal(b, &keys); err != nil || len(keys) == 0 {
				fmt.Printf("No API keys configured for %s\n", function)
				return nil
			}

			fmt.Printf("%-8s  %-24s  %s\n", "METHOD", "FUNCTION", "KEY")
			fmt.Printf("%-8s  %-24s  %s\n", "--------", "------------------------", "-------")
			for _, k := range keys {
				fmt.Printf("%-8s  %-24s  %s\n", k.Method, k.Path, k.Fingerprint)
			}
			return nil
		},
	}
}

// authRevoke removes the API key for a function+method.
func authRevoke() *cobra.Command {
	var method string

	cmd := &cobra.Command{
		Use:   "revoke <function-name>",
		Short: "Revoke the API key for a deployed function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			function := args[0]

			body, _ := json.Marshal(map[string]string{
				"function": function,
				"method":   strings.ToUpper(method),
			})

			resp, err := common.DoJSONRequest(
				http.MethodDelete,
				common.APIBaseURL+"/ops/atomic/auth",
				bytes.NewReader(body),
			)
			if err != nil {
				return fmt.Errorf("could not reach API: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(b))
			}

			fmt.Printf("API key revoked for %s %s\n", strings.ToUpper(method), function)
			return nil
		},
	}

	cmd.Flags().StringVarP(&method, "method", "m", "post", "HTTP method of the function (get, post, put, delete)")
	return cmd
}
