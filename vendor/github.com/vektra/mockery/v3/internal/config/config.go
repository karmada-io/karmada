package internal

import (
	"fmt"
	"os"

	"github.com/vektra/mockery/v3/internal/file"
)

func FindConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current working directory: %w", err)
	}
	configPath, _, err := file.FindInHierarchy(cwd, []string{".mockery.yaml", ".mockery.yml"})
	return configPath, err
}
