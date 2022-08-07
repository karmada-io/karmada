package utils

import (
	"os"
	"reflect"
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

func TestListFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "files",
			path: "../options",
			want: []string{"../options/global.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ListFiles(tt.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}
