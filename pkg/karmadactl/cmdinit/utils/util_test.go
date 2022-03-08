package utils

import (
	"fmt"
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

// TestGetInitVersion test GetInitVersion
func TestGetInitVersion(t *testing.T) {
	proVersion := GetInitVersion("v0.11.1")
	devVersion := GetInitVersion("v0.11.1-547-g0190fda9-dirty")
	fmt.Printf("[pro] image: %s crd: %s\n", proVersion.ImageVersion, proVersion.CRDVersion)
	fmt.Printf("[dev] image: %s crd: %s\n", devVersion.ImageVersion, devVersion.CRDVersion)
}
