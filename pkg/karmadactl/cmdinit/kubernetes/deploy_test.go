package kubernetes

import (
	"os"
	"testing"
)

func Test_initializeDirectory(t *testing.T) {
	tests := []struct {
		name                string
		createPathInAdvance bool
		wantErr             bool
	}{
		{
			name:                "Test when there is no dir exists",
			createPathInAdvance: false,
			wantErr:             false,
		},
		{
			name:                "Test when there is dir exists",
			createPathInAdvance: true,
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createPathInAdvance {
				if err := os.MkdirAll("tmp", os.FileMode(0755)); err != nil {
					t.Errorf("create test directory failed in advance:%v", err)
				}
			}
			if err := initializeDirectory("tmp"); (err != nil) != tt.wantErr {
				t.Errorf("initializeDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := os.RemoveAll("tmp"); err != nil {
				t.Errorf("clean up test directory failed after ut case:%s, %v", tt.name, err)
			}
		})
	}
}
