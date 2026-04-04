package atomic_cmd_new

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cli/common"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

//go:embed languages/golang_post.txt
var defaultGolangContentsPost string

//go:embed languages/golang_get.txt
var defaultGolangContentsGet string

//go:embed languages/golang_ws.txt
var defaultGolangContentsWS string


func Go() *cobra.Command {
	atomicNewCmd := &cobra.Command{
		Use:                "new",
		Short:              "Create a new Atomic function",
		GroupID:            "development",
		DisableFlagParsing: true,
		Args:               cobra.NoArgs,
		SilenceErrors:      true,
		Run: func(cmd *cobra.Command, args []string) {
			auth := "none"
			var endpointType, method, name string

			// Endpoint type
			typePrompt := &survey.Select{
				Message: "What type of endpoint?",
				Options: []string{"API Endpoint", "WebSocket Endpoint"},
				VimMode: true,
			}
			_ = survey.AskOne(typePrompt, &endpointType)

			if endpointType == "WebSocket Endpoint" {
				method = "ws"
			} else {
				// HTTP method
				methodPrompt := &survey.Select{
					Message: "Select HTTP method:",
					Options: []string{"POST", "GET", "PUT", "DELETE"},
					VimMode: true,
				}
				_ = survey.AskOne(methodPrompt, &method)
			}

			// Route name
			namePrompt := &survey.Input{
				Message: "Function route (e.g., users):",
			}
			_ = survey.AskOne(namePrompt, &name, survey.WithValidator(survey.Required))

			// Auth
			authPrompt := &survey.Select{
				Message: "Choose authentication:",
				Options: []string{"none", "apikey", "jwt"},
				VimMode: true,
			}
			_ = survey.AskOne(authPrompt, &auth)

			err := GenerateAtomicFunction(name, method, "go", auth)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}

			var integrations []string
			integrationPrompt := &survey.MultiSelect{
				Message: "Select integrations:",
				Options: []string{"shush", "backbone"},
				VimMode: true,
			}
			_ = survey.AskOne(integrationPrompt, &integrations)
			fmt.Printf("✅ Atomic function '%s' created with integrations: %v\n\n", name, integrations)

			fmt.Printf("🧪 Test it locally:\n\tdrift atomic run %s\n\n", name)
			fmt.Printf("🚀 Deploy it:\n\tdrift atomic deploy %s\n\n", name)
		},
	}

	atomicNewCmd.PersistentFlags().BoolP("help", "h", false, "")
	_ = atomicNewCmd.PersistentFlags().MarkHidden("help")

	return atomicNewCmd
}

func GenerateAtomicFunction(name, method, language, auth string) error {
	var handler string
	mainFile := filepath.Join(name, fmt.Sprintf("%s.go", name))
	dependenciesFile := filepath.Join(name, "go.mod")

	// WASM functions depend on drift-sdk; WebSocket falls back to native.
	var dependenciesFileContents string

	switch strings.ToLower(method) {
	case "post":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{METHOD_UPPER}}", common.CapitalizeFirst(strings.ToLower(method)),
			"{{NAME_UPPER}}", common.CapitalizeFirst(strings.ToLower(name)),
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultGolangContentsPost)
		dependenciesFileContents = fmt.Sprintf(
			"module atomic/%s\n\ngo 1.25\n\nrequire drift-sdk v0.0.0\n",
			name,
		)
	case "get":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{METHOD_UPPER}}", common.CapitalizeFirst(strings.ToLower(method)),
			"{{NAME_UPPER}}", common.CapitalizeFirst(strings.ToLower(name)),
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultGolangContentsGet)
		dependenciesFileContents = fmt.Sprintf(
			"module atomic/%s\n\ngo 1.25\n\nrequire drift-sdk v0.0.0\n",
			name,
		)
	case "ws":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{NAME_UPPER}}", common.CapitalizeFirst(strings.ToLower(name)),
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultGolangContentsWS)
		dependenciesFileContents = fmt.Sprintf(
			"module atomic/%s\n\ngo 1.25\n\nrequire github.com/gorilla/websocket v1.5.3\n",
			name,
		)
	}

	if err := os.MkdirAll(name, 0o750); err != nil {
		return fmt.Errorf("failed to create function directory: %w", err)
	}

	if err := os.WriteFile(mainFile, []byte(handler), 0o600); err != nil {
		return fmt.Errorf("failed to write main file: %w", err)
	}

	if err := os.WriteFile(dependenciesFile, []byte(dependenciesFileContents), 0o600); err != nil {
		return fmt.Errorf("failed to write dependencies file: %w", err)
	}

	dotEnv := "# Secrets for local development — loaded automatically by 'drift atomic run'\n# These values are pushed to Backbone Secrets on 'drift atomic deploy'\n#\n# Example:\n# DATABASE_URL=postgres://localhost:5432/mydb\n# API_KEY=your-api-key-here\n"
	if err := os.WriteFile(filepath.Join(name, ".env"), []byte(dotEnv), 0o600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	gitignore := ".env\n"
	if err := os.WriteFile(filepath.Join(name, ".gitignore"), []byte(gitignore), 0o600); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	return nil
}
