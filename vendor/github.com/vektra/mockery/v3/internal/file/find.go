package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

func CleanPath(filePath string) (string, error) {
	filePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("making %q an absolute path: %w", filePath, err)
	}
	return filepath.ToSlash(filePath), nil
}

// FindInHierarchy looks for a file with any of the names in the hierarchy up to the root folder.
// It returns the complete path of the file, the content and an error.
func FindInHierarchy(folder string, names []string) (string, []byte, error) {
	folder, err := CleanPath(folder)
	if err != nil {
		return "", nil, err
	}
	for {
		for _, name := range names {
			filePath := path.Join(folder, name)
			file, err := os.Open(filePath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return "", nil, fmt.Errorf("checking if %q exists: %w", filePath, err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return "", nil, fmt.Errorf("checking %q type: %w", filePath, err)
			}
			if !info.Mode().IsRegular() {
				return "", nil, fmt.Errorf("%q is not a regular file", filePath)
			}
			content, err := io.ReadAll(file)
			if err != nil {
				return "", nil, fmt.Errorf("reading %q: %w", filePath, err)
			}
			return filePath, content, nil
		}
		parent := path.Dir(folder)
		if folder == parent {
			return "", nil, errors.New("file not found")
		}
		folder = parent
	}
}
