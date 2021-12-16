package utils

import (
	"testing"
)

const filePath = "./crds.tar.gz"

//TestDownloadFile test DownloadFile
func TestDownloadFile(t *testing.T) {
	url := "https://github.com/karmada-io/karmada/releases/download/v0.9.0/crds.tar.gz"
	if err := DownloadFile(url, filePath); err != nil {
		panic(err)
	}

	if err := DeCompress(filePath, "./"); err != nil {
		panic(err)
	}
}
