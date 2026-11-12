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

package util

import (
	"os"
	"strings"
	"testing"
)

// substituteStdin replaces os.Stdin with a pipe seeded with input and returns
// a cleanup func that restores the original stdin.
func substituteStdin(t *testing.T, input string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	if _, err = w.WriteString(input); err != nil {
		t.Fatalf("failed to write to pipe: %v", err)
	}
	w.Close()

	orig := os.Stdin
	os.Stdin = r
	return func() {
		os.Stdin = orig
		r.Close()
	}
}

func TestDeleteConfirmation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOk  bool
		wantErr bool
	}{
		{
			name:    "DeleteConfirmation_WithYes_ReturnsTrue",
			input:   "yes\n",
			wantOk:  true,
			wantErr: false,
		},
		{
			name:    "DeleteConfirmation_WithY_ReturnsTrue",
			input:   "y\n",
			wantOk:  true,
			wantErr: false,
		},
		{
			name:    "DeleteConfirmation_WithNo_ReturnsFalse",
			input:   "no\n",
			wantOk:  false,
			wantErr: false,
		},
		{
			name:    "DeleteConfirmation_WithN_ReturnsFalse",
			input:   "n\n",
			wantOk:  false,
			wantErr: false,
		},
		{
			name:    "DeleteConfirmation_WithUppercaseYES_ReturnsTrue",
			input:   "YES\n",
			wantOk:  true,
			wantErr: false,
		},
		{
			name:    "DeleteConfirmation_WithUppercaseNO_ReturnsFalse",
			input:   "NO\n",
			wantOk:  false,
			wantErr: false,
		},
		{
			name:    "DeleteConfirmation_WithEmptyInput_ReturnsError",
			input:   "",
			wantOk:  false,
			wantErr: true,
		},
		{
			// Exercises the retry loop: first input is unrecognized,
			// second input is valid. Relies on the iterative for-loop
			// rather than recursion.
			name:    "DeleteConfirmation_WithInvalidThenYes_ReturnsTrue",
			input:   "maybe\nyes\n",
			wantOk:  true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := substituteStdin(t, tt.input)
			defer restore()

			got, err := DeleteConfirmation()
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteConfirmation() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantOk {
				t.Errorf("DeleteConfirmation() = %v, want %v", got, tt.wantOk)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "failed to read user confirmation") {
				t.Errorf("expected error to contain 'failed to read user confirmation', got: %v", err)
			}
		})
	}
}
