package atomic_common

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func ParseAtomicMetadataFromDir(dir string) (method, name, auth string, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read dir: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := filepath.Join(dir, file.Name())
		method, name, auth, err = ParseAtomicMetadata(filename)
		if err == nil {
			// Successfully parsed metadata from this file
			return method, name, auth, nil
		}
		// else ignore parse errors and try next file
	}

	return "", "", "", fmt.Errorf("no valid atomic metadata found in directory %s", dir)
}

func ParseAtomicMetadata(filename string) (method, name, auth string, err error) {
	data, err := os.ReadFile(filename) // #nosec G304 — CLI tool reads user's own source files by design
	if err != nil {
		return "", "", "", err
	}

	re := regexp.MustCompile(`@atomic route=([a-zA-Z]+):([a-zA-Z0-9_/:-]+) auth=([a-zA-Z0-9_-]+)`)
	matches := re.FindSubmatch(data)
	if len(matches) != 4 {
		return "", "", "", fmt.Errorf("atomic metadata not found or invalid format")
	}

	method = string(matches[1])
	name = string(matches[2])
	auth = string(matches[3])
	return method, name, auth, nil
}
