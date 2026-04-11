package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
)

func DoLogin(username, password string) {
	jsonData, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Post(common.APIBaseURL+"/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println(common.TransportError("log in", err))
		return
	}
	defer resp.Body.Close()

	body, err := common.CheckResponse(resp, "log in")
	if err != nil {
		fmt.Println(err)
		return
	}

	var respData map[string]string
	if err := json.Unmarshal(body, &respData); err != nil {
		fmt.Println("Couldn't log in: the API response didn't look right —", err)
		return
	}

	token := respData["access_token"]
	refreshToken := respData["refresh_token"]
	if token == "" || refreshToken == "" {
		fmt.Println("Couldn't log in: the API didn't return a full set of tokens. That's on us; please try again.")
		return
	}

	if err := common.SaveSession(token, refreshToken); err != nil {
		fmt.Println("Logged in, but couldn't save your session to disk:", err)
		return
	}
	fmt.Printf("Logged in as %s.\n", username)
}

func GetLoginCmd() *cobra.Command {
	var username, password string

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Drift and get a JWT token",
		Example: `  drift account login
  drift account login --username alice
  drift account login -u alice -p s3cret`,
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
