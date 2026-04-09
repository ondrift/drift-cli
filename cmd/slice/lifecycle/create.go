package lifecycle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"cli/common"

	"github.com/spf13/cobra"
)

// getCreateCmd builds `drift slice create [name]`.
//
// Default flow: `drift slice create` (no args) opens the configurator in
// a browser. The user types the slice name into the form, picks a
// SliceConfig (or a preset), reviews the price, and submits — the
// configurator forwards the create call to api on the user's behalf and
// the CLI just polls for the result. Passing a positional name still
// works as a convenience: it pre-fills the form's name field.
//
// Headless flow: --headless skips the browser entirely and posts directly
// to api/ops/slice/create with whatever tier the user named on the
// command line. The name is required in headless mode because there is
// no form to collect it from. This path is the only one that works in
// CI, scripts, and SSH sessions, but it is restricted to named tiers
// (no per-resource tweaking) because the CLI does not implement a config
// builder.
func getCreateCmd() *cobra.Command {
	var (
		tier     string
		headless bool
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new slice (project)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var name string
			if len(args) == 1 {
				name = args[0]
			}

			if headless {
				if name == "" {
					fmt.Println("Couldn't create slice: --headless requires a slice name argument.")
					return
				}
				if err := createHeadless(name, tier); err != nil {
					fmt.Println(err)
					return
				}
				return
			}

			// Browser flow. The configurator handles the entire form
			// interaction; the CLI just opens the URL and waits for
			// the user to finish. The name (if any) is forwarded as a
			// pre-fill hint — the form is still the source of truth.
			result, err := runBrowserHandoff("create slice", name, modeCreate, nil)
			if err != nil {
				fmt.Println(err)
				return
			}

			// The configurator returns the full Slice document, which
			// is the only place where the actually-created name lives
			// in the no-args flow. Pull it out before saving as active.
			activeName := sliceNameFromResult(result, name)
			if activeName != "" {
				if err := common.SaveActiveSlice(activeName); err != nil {
					fmt.Println("Warning: couldn't mark the new slice as active —", err)
				}
			}
			printSliceSummary("created and set as active", result)
		},
	}

	cmd.Flags().StringVarP(&tier, "tier", "t", "hacker", "Named tier (hacker, prototyper, ...). Only used with --headless.")
	cmd.Flags().BoolVar(&headless, "headless", false, "Skip the browser configurator and create the slice with the named tier")
	return cmd
}

// sliceNameFromResult extracts the "name" field out of the Slice document
// returned by the configurator. The fallback is the name the user passed
// on the command line (if any), so the active-slice marker is set even
// when the result body is missing or unparseable for some reason.
func sliceNameFromResult(raw json.RawMessage, fallback string) string {
	var s struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &s); err == nil && s.Name != "" {
		return s.Name
	}
	return fallback
}

// createHeadless posts directly to api/ops/slice/create with a named tier.
// It is the original pre-configurator flow, kept for non-interactive use.
// We deliberately do not accept a SliceConfig file here: a JSON-driven
// CLI builder is its own design problem and the browser flow is the
// expected path for any user who needs more than the default tiers.
func createHeadless(name, tier string) error {
	if tier == "" {
		tier = "hacker"
	}
	body, _ := json.Marshal(map[string]string{
		"name": name,
		"tier": tier,
	})

	resp, err := common.DoJSONRequest(
		http.MethodPost,
		common.APIBaseURL+"/ops/slice/create",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return common.TransportError("create slice", err)
	}
	defer resp.Body.Close()

	if _, err := common.CheckResponse(resp, "create slice"); err != nil {
		return err
	}

	if err := common.SaveActiveSlice(name); err != nil {
		fmt.Println("Warning: couldn't mark the new slice as active —", err)
	}
	fmt.Printf("Slice '%s' created and set as active.\n", name)
	return nil
}
