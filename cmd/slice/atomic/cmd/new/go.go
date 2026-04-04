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

//go:embed languages/python_post.txt
var defaultPythonContentsPost string

//go:embed languages/python_get.txt
var defaultPythonContentsGet string

//go:embed languages/python_ws.txt
var defaultPythonContentsWS string

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
			var lang, endpointType, method, name string

			// Language
			langPrompt := &survey.Select{
				Message: "Select language:",
				Options: []string{"Go", "Python"},
				VimMode: true,
			}
			_ = survey.AskOne(langPrompt, &lang)
			if lang == "Go" {
				lang = "go"
			} else {
				lang = "python"
			}

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

			var err error
			if lang == "python" {
				err = GenerateAtomicFunctionPython(name, method, auth)
			} else {
				err = GenerateAtomicFunction(name, method, lang, auth)
			}
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}

			// Integrations (Go only — Python uses vendor/ instead)
			if lang == "go" {
				var integrations []string
				integrationPrompt := &survey.MultiSelect{
					Message: "Select integrations:",
					Options: []string{"shush", "backbone"},
					VimMode: true,
				}
				_ = survey.AskOne(integrationPrompt, &integrations)
				fmt.Printf("✅ Atomic function '%s' created with integrations: %v\n\n", name, integrations)
			} else {
				fmt.Printf("✅ Atomic function '%s' created (Python)\n\n", name)
			}

			fmt.Printf("🧪 Test it locally:\n\tdrift atomic run %s\n\n", name)
			fmt.Printf("🚀 Deploy it:\n\tdrift atomic deploy %s\n\n", name)
		},
	}

	atomicNewCmd.PersistentFlags().BoolP("help", "h", false, "")
	_ = atomicNewCmd.PersistentFlags().MarkHidden("help")

	return atomicNewCmd
}

func GenerateAtomicFunctionPython(name, method, auth string) error {
	var handler string
	mainFile := filepath.Join(name, "handler.py")

	nameUpper := common.CapitalizeFirst(strings.ToLower(name))

	switch strings.ToLower(method) {
	case "post":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{NAME_UPPER}}", nameUpper,
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultPythonContentsPost)
	case "get":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{NAME_UPPER}}", nameUpper,
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultPythonContentsGet)
	case "ws":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{NAME_UPPER}}", nameUpper,
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultPythonContentsWS)
	}

	if err := os.MkdirAll(name, 0o750); err != nil {
		return fmt.Errorf("failed to create function directory: %w", err)
	}

	if err := os.WriteFile(mainFile, []byte(handler), 0o600); err != nil {
		return fmt.Errorf("failed to write handler.py: %w", err)
	}

	// requirements.txt — websockets only needed for WS endpoints
	var requirements string
	if strings.ToLower(method) == "ws" {
		requirements = "websockets>=12.0\n"
	}
	if err := os.WriteFile(filepath.Join(name, "requirements.txt"), []byte(requirements), 0o600); err != nil {
		return fmt.Errorf("failed to write requirements.txt: %w", err)
	}

	dotEnv := "# Secrets for local development — loaded automatically by 'drift atomic run'\n# These values are pushed to Backbone Secrets on 'drift atomic deploy'\n#\n# Example:\n# DATABASE_URL=postgres://localhost:5432/mydb\n# API_KEY=your-api-key-here\n"
	if err := os.WriteFile(filepath.Join(name, ".env"), []byte(dotEnv), 0o600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	gitignore := ".env\nvendor/\n__pycache__/\n"
	if err := os.WriteFile(filepath.Join(name, ".gitignore"), []byte(gitignore), 0o600); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	return nil
}

func GenerateAtomicFunction(name, method, language, auth string) error {
	var handler string
	mainFile := filepath.Join(name, fmt.Sprintf("%s.go", name))
	dependenciesFile := filepath.Join(name, "go.mod")
	dependenciesFileContents := fmt.Sprintf("module atomic/%s\n\ngo 1.25.5", name)

	switch strings.ToLower(method) {
	case "post":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{METHOD_UPPER}}", common.CapitalizeFirst(strings.ToLower(method)),
			"{{NAME_UPPER}}", common.CapitalizeFirst(strings.ToLower(name)),
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultGolangContentsPost)
	case "get":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{METHOD_UPPER}}", common.CapitalizeFirst(strings.ToLower(method)),
			"{{NAME_UPPER}}", common.CapitalizeFirst(strings.ToLower(name)),
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultGolangContentsGet)
	case "ws":
		replacer := strings.NewReplacer(
			"{{NAME}}", name,
			"{{NAME_UPPER}}", common.CapitalizeFirst(strings.ToLower(name)),
			"{{AUTH}}", auth,
		)
		handler = replacer.Replace(defaultGolangContentsWS)
		dependenciesFileContents = fmt.Sprintf(
			"module atomic/%s\n\ngo 1.25.5\n\nrequire github.com/gorilla/websocket v1.5.3\n",
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
