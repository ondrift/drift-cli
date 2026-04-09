package common

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser launches the user's default web browser at the given URL.
// We avoid pulling in a third-party dependency for this because the per-OS
// command is a one-liner: macOS uses `open`, Linux uses `xdg-open`, and
// Windows shells out to `cmd /c start`. If the launch fails (no display,
// SSH session, container, locked-down environment) we return an error so
// the caller can fall back to printing the URL for the user to open
// manually — that fallback is the whole reason this function returns an
// error rather than panicking.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) // #nosec G204 — url comes from a Drift-controlled handoff response
	case "linux":
		cmd = exec.Command("xdg-open", url) // #nosec G204 — same as above
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url) // #nosec G204 — same as above
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}
