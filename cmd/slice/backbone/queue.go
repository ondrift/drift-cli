package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"cli/common"

	"github.com/spf13/cobra"
)

func queueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Push and pop messages from named queues in your slice",
	}
	cmd.AddCommand(queuePushCmd(), queuePopCmd(), queuePeekCmd(), queueLenCmd(), queueDropCmd())
	return cmd
}

func queuePushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <name> <json>",
		Short: "Push a JSON message onto a queue",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			name, rawJSON := args[0], args[1]

			var body map[string]any
			if err := json.Unmarshal([]byte(rawJSON), &body); err != nil {
				fmt.Println("Couldn't push message: that doesn't look like valid JSON —", err)
				return
			}

			payload, _ := json.Marshal(map[string]any{"queue": name, "body": body})
			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/backbone/queue/push",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println(common.TransportError("push message", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "push message")
			if err != nil {
				fmt.Println(err)
				return
			}

			var result map[string]string
			if err := json.Unmarshal(b, &result); err == nil {
				fmt.Printf("Message pushed (id: %s)\n", result["id"])
			} else {
				fmt.Println("Message pushed")
			}
		},
	}
}

func queuePopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pop <name>",
		Short: "Pop the next message from a queue",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			url := fmt.Sprintf("%s/ops/backbone/queue/pop?queue=%s", common.APIBaseURL, name)
			resp, err := common.DoRequest(http.MethodPost, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("pop message", err))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				fmt.Println("Queue is empty.")
				return
			}

			b, err := common.CheckResponse(resp, "pop message")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(string(b))
		},
	}
}

func queuePeekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "peek <name>",
		Short: "Peek at the next message without removing it",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			url := fmt.Sprintf("%s/ops/backbone/queue/peek?queue=%s", common.APIBaseURL, name)
			resp, err := common.DoRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("peek queue", err))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				fmt.Println("Queue is empty.")
				return
			}

			b, err := common.CheckResponse(resp, "peek queue")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(string(b))
		},
	}
}

func queueDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop <name>",
		Short: "Delete a queue and all its messages",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			url := fmt.Sprintf("%s/ops/backbone/queue/drop?queue=%s", common.APIBaseURL, name)
			resp, err := common.DoRequest(http.MethodPost, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("drop queue", err))
				return
			}
			defer resp.Body.Close()

			if _, err := common.CheckResponse(resp, "drop queue"); err != nil {
				fmt.Println(err)
				return
			}
			fmt.Printf("Queue %q dropped\n", name)
		},
	}
}

func queueLenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "len <name>",
		Short: "Print the number of messages in a queue",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			url := fmt.Sprintf("%s/ops/backbone/queue/len?queue=%s", common.APIBaseURL, name)
			resp, err := common.DoRequest(http.MethodGet, url, nil)
			if err != nil {
				fmt.Println(common.TransportError("get queue length", err))
				return
			}
			defer resp.Body.Close()

			b, err := common.CheckResponse(resp, "get queue length")
			if err != nil {
				fmt.Println(err)
				return
			}

			var result map[string]int
			if err := json.Unmarshal(b, &result); err == nil {
				fmt.Println(result["length"])
			} else {
				fmt.Println(string(b))
			}
		},
	}
}
