package slice

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cli/common"

	"github.com/spf13/cobra"
)

// getResizeCmd builds `drift slice resize <name>`.
//
// Resize is browser-only: changing a SliceConfig is exactly the kind of
// review-and-confirm step the configurator was built for, and a CLI form
// for "raise function count from 5 to 7, drop one queue, add a vector
// collection" would be miserable to use. We deliberately do not provide
// a --headless mode for resize. Tests that need to drive resize should
// hit /ops/slice/resize directly.
//
// The handoff carries the existing SliceConfig so the form can pre-fill
// itself; we fetch it from /ops/slice/get on the api side rather than
// caching it locally because the source of truth is in MongoDB.
func getResizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resize <name>",
		Short: "Resize an existing slice (open the configurator in your browser)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			existing, err := fetchSliceConfig(name)
			if err != nil {
				fmt.Println(err)
				return
			}

			result, err := runBrowserHandoff("resize slice", name, modeResize, existing)
			if err != nil {
				fmt.Println(err)
				return
			}
			printSliceSummary("resized", result)
		},
	}
	return cmd
}

// fetchSliceConfig pulls the user's current SliceConfig from api so the
// configurator form can pre-populate. We return the JSON-decoded value as
// a generic any so the handoff helper can re-encode it without the CLI
// having to import drift-common/models. The shape is intentionally
// passthrough — the CLI never inspects the config, it only forwards it.
func fetchSliceConfig(name string) (any, error) {
	resp, err := common.DoRequest(
		http.MethodGet,
		common.APIBaseURL+"/ops/slice/get?name="+name,
		nil,
	)
	if err != nil {
		return nil, common.TransportError("resize slice", err)
	}
	defer resp.Body.Close()

	body, err := common.CheckResponse(resp, "resize slice")
	if err != nil {
		return nil, err
	}

	// /ops/slice/get returns the full Slice document; the configurator
	// only needs the embedded "config" subobject. Pull it out so the
	// handoff payload matches the configurator's handoffRequest.Existing
	// field shape.
	var slice struct {
		Config any `json:"config"`
	}
	if err := json.Unmarshal(body, &slice); err != nil {
		return nil, fmt.Errorf("Couldn't resize slice: get response wasn't valid JSON (%w)", err)
	}
	if slice.Config == nil {
		return nil, fmt.Errorf("Couldn't resize slice: server returned no config for slice %q", name)
	}
	return slice.Config, nil
}
