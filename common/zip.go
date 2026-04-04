package common

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func ZipFolder(folderPath string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create the header based on file info
		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}
		// Skip root folder
		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		// Use relative path in zip archive
		header.Name = relPath

		// For directories, just create the folder entry
		if info.IsDir() {
			header.Name += "/"
		} else {
			// Use deflate compression for files
			header.Method = zip.Deflate
		}

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		// If file, copy contents into zip writer
		if !info.IsDir() {
			f, err := os.Open(path) // #nosec G304 — CLI tool reads user's own project files by design
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(writer, f)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		_ = zw.Close()
		return nil, err
	}

	err = zw.Close()
	if err != nil {
		return nil, err
	}

	return buf, nil
}
