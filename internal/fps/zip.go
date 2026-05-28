package fps

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip extracts a ZIP archive's test data files into targetDir.
// Returns the content of any problem.xml or fps.xml found in the root of the zip.
// Files with path traversal (..) or directory separators are skipped.
func ExtractZip(zipData []byte, targetDir string) (xmlContent []byte, err error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip archive: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	for _, file := range reader.File {
		name := filepath.Base(file.Name) // get only the filename, strip any dirs

		// Reject unsafe filenames
		if strings.Contains(file.Name, "..") {
			continue
		}

		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open entry %s: %w", file.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read entry %s: %w", file.Name, err)
		}

		// Capture XML content
		if name == "problem.xml" || name == "fps.xml" {
			xmlContent = data
			continue
		}

		// Write test data files into targetDir
		destPath := filepath.Join(targetDir, name)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", name, err)
		}
	}

	return xmlContent, nil
}
