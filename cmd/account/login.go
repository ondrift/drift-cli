package account

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

func DoLogin(username, password string) {
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Println("❌ Failed to marshal JSON:", err)
		return
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Post("http://api.localhost:30036/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("❌ Failed to send login request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ Login failed: %s\n", string(bodyBytes))
		return
	}

	var respData map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		fmt.Println("❌ Failed to parse response:", err)
		return
	}

	token, ok := respData["access_token"]
	if !ok {
		fmt.Println("❌ No token found in response")
		return
	}

	refresh_token, ok := respData["refresh_token"]
	if !ok {
		fmt.Println("❌ No refresh token found in response")
		return
	}

	err = common.SaveSession(token, refresh_token)
	if err != nil {
		fmt.Println("❌ Failed to save token:", err)
		return
	}
}

func GetLoginCmd() *cobra.Command {
	var username, password string

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Drift and get a JWT token",
		Run: func(cmd *cobra.Command, args []string) {
			if username == "" {
				username = common.PromptForInput("Username")
			}
			if password == "" {
				password = common.PromptForInputHidden("Password")
			}
			DoLogin(username, password)
		},
	}

	loginCmd.Flags().StringVarP(&username, "username", "u", "", "Username (skips interactive prompt)")
	loginCmd.Flags().StringVarP(&password, "password", "p", "", "Password (skips interactive prompt)")

	return loginCmd
}
