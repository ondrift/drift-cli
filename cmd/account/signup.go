package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"cli/common"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func GetSignupCmd() *cobra.Command {
	var username, password, email string

	signupCmd := &cobra.Command{
		Use:   "signup",
		Short: "Signup to Drift",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if username == "" {
				username = common.PromptForInput("Username")
			}
			if email == "" {
				email = common.PromptForInput("Email")
			}
			if password == "" {
				password = common.PromptForInputHidden("Password")
			}
			repeatPassword := password
			if !cmd.Flags().Changed("password") {
				repeatPassword = common.PromptForInputHidden("Repeat password")
			}

			if password != repeatPassword {
				fmt.Println("❌ Passwords don't match")
				return
			}

			hashedPassword, err := HashPassword(password)
			if err != nil {
				fmt.Println("❌ Failed to hash password")
				return
			}

			// Step 1: initiate signup — sends OTP to the user's email.
			fmt.Println("\nSending verification code...")

			initiatePayload, _ := json.Marshal(map[string]string{
				"username": username,
				"password": hashedPassword,
				"email":    email,
			})

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Post(common.APIBaseURL+"/signup/initiate", "application/json", bytes.NewBuffer(initiatePayload))
			if err != nil {
				log.Fatalf("request failed: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				fmt.Printf("❌ Signup failed: %s\n", string(body))
				return
			}

			// Step 2: prompt for OTP and verify.
			// In dev mode (DRIFT_ENV=local on the server) the code is always
			// "000000" and the CLI skips the prompt entirely.
			var initiateResp struct {
				DevMode bool `json:"dev_mode"`
			}
			_ = json.Unmarshal(body, &initiateResp)

			var code string
			if initiateResp.DevMode {
				code = "000000"
				fmt.Println("⚡ Dev mode — skipping email verification")
			} else {
				fmt.Println("✅ Check your email for a 6-digit verification code.")
				code = common.PromptForInput("Verification code")
			}

			verifyPayload, _ := json.Marshal(map[string]string{
				"username": username,
				"code":     code,
			})

			resp, err = client.Post(common.APIBaseURL+"/signup/verify", "application/json", bytes.NewBuffer(verifyPayload))
			if err != nil {
				log.Fatalf("verify request failed: %v", err)
			}
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				fmt.Printf("❌ Verification failed: %s\n", string(body))
				return
			}

			// Parse tokens and save session.
			var tokenResp struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.Unmarshal(body, &tokenResp); err != nil || tokenResp.AccessToken == "" {
				fmt.Println("❌ Failed to parse token response")
				return
			}
			if err := common.SaveSession(tokenResp.AccessToken, tokenResp.RefreshToken); err != nil {
				fmt.Println("❌ Failed to save session:", err)
				return
			}

			fmt.Printf("\n\033[48;2;241;160;6m"+" "+"\033[0m"+" Welcome to Drift, %s!\n", username)
			fmt.Println("\n\033[48;2;61;213;166m" + " " + "\033[0m" + " Next steps:")
			fmt.Println("  1. Create your first slice (project)            :: 'drift slice create <name>'")
			fmt.Println("  2. Deploy your app                              :: 'drift deploy drift.yaml'")
			fmt.Println("\nManage slices                                    :: 'drift slice list'")
			fmt.Println("Switch active slice                               :: 'drift slice use <name>'")
			fmt.Println("Check your usage anytime                          :: 'drift account usage'")
			fmt.Println("Happy building!")
		},
		Example: "drift signup",
	}

	signupCmd.Flags().StringVarP(&username, "username", "u", "", "Username (skips interactive prompt)")
	signupCmd.Flags().StringVarP(&email, "email", "e", "", "Email address (skips interactive prompt)")
	signupCmd.Flags().StringVarP(&password, "password", "p", "", "Password (skips interactive prompt and repeat)")

	return signupCmd
}
