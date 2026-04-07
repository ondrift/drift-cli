package backbone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
				fmt.Println("❌ Invalid JSON body:", err)
				return
			}

			payload, _ := json.Marshal(map[string]any{"queue": name, "body": body})
			resp, err := common.DoJSONRequest(
				http.MethodPost,
				common.APIBaseURL+"/ops/backbone/queue/push",
				bytes.NewBuffer(payload),
			)
			if err != nil {
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to push message: %s\n", string(b))
				return
			}

			var result map[string]string
			if err := json.Unmarshal(b, &result); err == nil {
				fmt.Printf("✅ Message pushed (id: %s)\n", result["id"])
			} else {
				fmt.Println("✅ Message pushed")
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
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				fmt.Println("Queue is empty.")
				return
			}

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to pop message: %s\n", string(b))
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
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				fmt.Println("Queue is empty.")
				return
			}

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to peek queue: %s\n", string(b))
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
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				fmt.Printf("❌ Failed to drop queue: %s\n", string(b))
				return
			}
			fmt.Printf("✅ Queue %q dropped\n", name)
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
				fmt.Println("❌ Failed to contact API:", err)
				return
			}
			defer resp.Body.Close()

			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("❌ Failed to get queue length: %s\n", string(b))
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
