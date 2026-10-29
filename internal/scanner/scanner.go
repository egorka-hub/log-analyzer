package scanner

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

func ScanLogDirectory(dirPath string) ([]string, error) {
	var logFiles []string

	err := filepath.WalkDir(dirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.HasSuffix(path, ".log") {
			logFiles = append(logFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %q: %w", dirPath, err)
	}

	return logFiles, nil
}
