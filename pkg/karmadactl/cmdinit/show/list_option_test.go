/*
Copyright 2024 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package show_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/show"
)

// Helper function to capture stdout
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestCommandConfigImageOption(t *testing.T) {
	tests := []struct {
		name                 string
		privateImageRegistry string
		expectedImages       []string
	}{
		{
			name:                 "Default execution",
			privateImageRegistry: "",
			expectedImages:       kubernetes.NewDefaultCommandInitOption().GenerateControlPlaneImages(),
		},
		{
			name:                 "With Private Image Registry",
			privateImageRegistry: "myregistry.com",
			expectedImages: func() []string {
				opts := kubernetes.NewDefaultCommandInitOption()
				opts.ImageRegistry = "myregistry.com"
				return opts.GenerateControlPlaneImages()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := show.CommandConfigImageOption{
				InitOption: kubernetes.NewDefaultCommandInitOption(),
			}
			opts.InitOption.ImageRegistry = tt.privateImageRegistry
			assert.NotNil(t, opts.InitOption)

			output := captureOutput(func() {
				err := opts.Run()
				assert.NoError(t, err)
			})

			for _, image := range tt.expectedImages {
				assert.Contains(t, output, image)
			}
		})
	}
}
