package utils

import (
	"os"
	"testing"
)

const (
	filePath = "./crds.tar.gz"
	folder   = "./crds"
)

// TestDownloadFile test DownloadFile
func TestDownloadFile(t *testing.T) {
	defer os.Remove(filePath)
	defer os.RemoveAll(folder)

	url := "https://github.com/karmada-io/karmada/releases/download/v0.9.0/crds.tar.gz"
	if err := DownloadFile(url, filePath); err != nil {
		panic(err)
	}

	if err := DeCompress(filePath, "./"); err != nil {
		panic(err)
	}
}
