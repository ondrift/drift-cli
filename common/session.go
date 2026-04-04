package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const SessionFile = "~/.drift/session.json"

type Session struct {
	Username string `json:"username"`
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
	path, err := expandPath(SessionFile)
	if err != nil {
		return err
	}

	// Make sure directory exists
	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return err
	}

	// Save token JSON
	data := map[string]string{"token": token, "refresh_token": refresh_token}
	f, err := os.Create(path) // #nosec G304 — CLI tool writes to user's own session file by design
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(data)
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
