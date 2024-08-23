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

package sharedcommand

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCmdVersion(t *testing.T) {
	testCases := []struct {
		name          string
		parentCommand string
		expectedUse   string
		expectedShort string
		expectedLong  string
	}{
		{
			name:          "karmada-controller-manager version command",
			parentCommand: "karmada-controller-manager",
			expectedUse:   "version",
			expectedShort: versionShort,
			expectedLong:  versionLong,
		},
		{
			name:          "karmadactl version command",
			parentCommand: "karmadactl",
			expectedUse:   "version",
			expectedShort: versionShort,
			expectedLong:  versionLong,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdVersion(tc.parentCommand)

			assert.Equal(t, tc.expectedUse, cmd.Use)
			assert.Equal(t, tc.expectedShort, cmd.Short)
			assert.Equal(t, tc.expectedLong, cmd.Long)
			assert.Equal(t, fmt.Sprintf(versionExample, tc.parentCommand), cmd.Example)

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd.Run(cmd, []string{})
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, err := io.Copy(&buf, r)
			if err != nil {
				t.Fatalf("Failed to copy: %v", err)
			}
			output := buf.String()

			assert.Contains(t, output, tc.parentCommand)
			assert.Contains(t, output, "version:")
			assert.Contains(t, output, "GitVersion:")
			assert.Contains(t, output, "GitCommit:")
			assert.Contains(t, output, "GitTreeState:")
			assert.Contains(t, output, "BuildDate:")
			assert.Contains(t, output, "GoVersion:")
			assert.Contains(t, output, "Compiler:")
			assert.Contains(t, output, "Platform:")
		})
	}
}

func TestNewCmdVersionOutput(t *testing.T) {
	oldStdout := os.Stdout

	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewCmdVersion("test-command")
	cmd.Run(cmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to copy: %v", err)
	}
	output := buf.String()

	assert.Contains(t, output, "test-command")
	assert.Contains(t, output, "version:")
	assert.Contains(t, output, "GitVersion:")
	assert.Contains(t, output, "GitCommit:")
	assert.Contains(t, output, "GitTreeState:")
	assert.Contains(t, output, "BuildDate:")
	assert.Contains(t, output, "GoVersion:")
	assert.Contains(t, output, "Compiler:")
	assert.Contains(t, output, "Platform:")
}

func TestNewCmdVersionHelp(t *testing.T) {
	cmd := NewCmdVersion("test-command")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	err := cmd.Help()
	if err != nil {
		t.Fatalf("Failed to execute Help: %v", err)
	}

	output := buf.String()
	assert.True(t, strings.Contains(output, "Usage:"))
	assert.True(t, strings.Contains(output, "test-command version"))
	assert.True(t, strings.Contains(output, versionShort))
	assert.True(t, strings.Contains(output, versionLong))
	assert.True(t, strings.Contains(output, "Examples:"))
	assert.True(t, strings.Contains(output, fmt.Sprintf(versionExample, "test-command")))
}

func TestNewCmdVersionFlags(t *testing.T) {
	cmd := NewCmdVersion("test-command")
	assert.Equal(t, 0, len(cmd.Flags().Args()))
}
