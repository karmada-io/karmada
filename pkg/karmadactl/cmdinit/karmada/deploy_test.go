/*
Copyright 2025 The Karmada Authors.

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

package karmada

import (
	"os"
	"testing"

	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestCrdPatchesResources(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		caBundle        string
		systemNs        string
		expectedContent string
	}{
		{
			name:            "simple replacement",
			content:         "caBundle: {{caBundle}}\nname: {{name}}\nnamespace: {{namespace}}",
			caBundle:        "testCaBundle",
			systemNs:        "testNamespace",
			expectedContent: "caBundle: testCaBundle\nname: " + names.KarmadaWebhookComponentName + "\nnamespace: testNamespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "example")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write([]byte(tt.content)); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}
			if err := tmpFile.Close(); err != nil {
				t.Fatalf("failed to close temp file: %v", err)
			}

			result, err := crdPatchesResources(tmpFile.Name(), tt.caBundle, tt.systemNs)
			if err != nil {
				t.Errorf("crdPatchesResources() error = %v", err)
			}

			if string(result) != tt.expectedContent {
				t.Errorf("crdPatchesResources() = %v, want %v", string(result), tt.expectedContent)
			}
		})
	}
}
