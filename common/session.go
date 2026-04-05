package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const SessionFile = "~/.drift/session.json"

// APIBaseURL is the base URL for the Drift API gateway.
const APIBaseURL = "http://api.localhost:30036"

type Session struct {
	Username    string `json:"username"`
	ActiveSlice string `json:"active_slice,omitempty"`
}

func expandPath(path string) (string, error) {
	if len(path) > 0 && path[:2] == "~/" {
		// Use $HOME directly so that tests (and tools that override HOME) work
		// correctly. CGo's user.Current() ignores the HOME env var on macOS.
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME environment variable is not set")
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func SaveSession(token, refresh_token string) error {
	// Read existing session to preserve active_slice across logins.
	data, _ := readSessionMap()
	if data == nil {
		data = make(map[string]string)
	}
	data["token"] = token
	data["refresh_token"] = refresh_token
	return writeSessionMap(data)
}

func GetTokenFromSession() (token string, refreshToken string, err error) {
	// Get full path to session file
	path, err := expandPath(SessionFile)
	if err != nil {
		return "", "", err
	}

	// Open the file for reading
	f, err := os.Open(path) // #nosec G304 — CLI tool reads user's own session file by design
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	// Decode JSON into a map or struct
	data := make(map[string]string)
	dec := json.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return "", "", err
	}

	// Extract tokens from the map
	token, ok1 := data["token"]
	refreshToken, ok2 := data["refresh_token"]
	if !ok1 || !ok2 {
		return "", "", fmt.Errorf("token or refresh_token not found in session file")
	}

	return token, refreshToken, nil
}

// readSessionMap loads the raw session JSON as a string map.
func readSessionMap() (map[string]string, error) {
	path, err := expandPath(SessionFile)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := make(map[string]string)
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// writeSessionMap persists the raw session JSON.
func writeSessionMap(data map[string]string) error {
	path, err := expandPath(SessionFile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.Create(path) // #nosec G304
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(data)
}

// SaveActiveSlice persists the active slice name into the session file.
func SaveActiveSlice(name string) error {
	data, err := readSessionMap()
	if err != nil {
		return fmt.Errorf("no active session — log in first")
	}
	data["active_slice"] = name
	return writeSessionMap(data)
}

// GetActiveSlice returns the active slice name, or empty string if none set.
func GetActiveSlice() string {
	data, err := readSessionMap()
	if err != nil {
		return ""
	}
	return data["active_slice"]
}

// RequireActiveSlice returns the active slice or an error instructing the user
// to select one with "drift slice use <name>".
func RequireActiveSlice() (string, error) {
	s := GetActiveSlice()
	if s == "" {
		return "", fmt.Errorf("no active slice — run 'drift slice use <name>' or 'drift slice create <name>' first")
	}
	return s, nil
}
