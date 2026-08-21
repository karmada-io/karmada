/*
Copyright 2026 The Karmada Authors.

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

package genericresource

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile writes content into a new file under dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

// TestBuilderDefaultConstructor covers the object type used when the caller does
// not call Constructor. json.Unmarshal rejects a non-pointer target, so a default
// that is not a pointer fails on every document.
func TestBuilderDefaultConstructor(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\n")

	objects, err := NewBuilder().Filename(false, path).Do().Objects()
	require.NoError(t, err)
	require.Len(t, objects, 1)

	obj, ok := objects[0].(*map[string]any)
	require.True(t, ok, "unexpected object type %T", objects[0])
	assert.Equal(t, "ConfigMap", (*obj)["kind"])
}

func TestBuilderConstructor(t *testing.T) {
	type configMap struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}

	dir := t.TempDir()
	path := writeFile(t, dir, "cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\n")

	objects, err := NewBuilder().
		Constructor(func() any { return &configMap{} }).
		Filename(false, path).
		Do().
		Objects()
	require.NoError(t, err)
	require.Len(t, objects, 1)

	obj, ok := objects[0].(*configMap)
	require.True(t, ok, "unexpected object type %T", objects[0])
	assert.Equal(t, "ConfigMap", obj.Kind)
	assert.Equal(t, "sample", obj.Metadata.Name)
}

func TestBuilderErrors(t *testing.T) {
	dir := t.TempDir()
	valid := writeFile(t, dir, "cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\n")

	tests := []struct {
		name         string
		build        func() *Builder
		wantErrMatch string
	}{
		{
			name:         "no input at all",
			build:        func() *Builder { return NewBuilder() },
			wantErrMatch: "you must provide one or more resources",
		},
		{
			name:         "missing file",
			build:        func() *Builder { return NewBuilder().Filename(false, filepath.Join(dir, "absent.yaml")) },
			wantErrMatch: "does not exist",
		},
		{
			// Stdin can only be consumed once.
			name:         "stdin referenced twice",
			build:        func() *Builder { return NewBuilder().Filename(false, "-", "-") },
			wantErrMatch: "standard input cannot be used for multiple arguments",
		},
		{
			name:         "malformed url",
			build:        func() *Builder { return NewBuilder().Filename(false, "http://[::1") },
			wantErrMatch: "is not valid",
		},
		{
			name: "single valid file",
			// Included as a control: this one must succeed.
			build:        func() *Builder { return NewBuilder().Filename(false, valid) },
			wantErrMatch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.build().Do().Err()
			if tt.wantErrMatch == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMatch)
		})
	}
}

// TestBuilderDirectoryRecursion covers passing a directory rather than a file.
// ExpandPathsToFileVisitors walks the tree, and without recursion it returns
// filepath.SkipDir for every directory below the one it was given, so files in a
// subdirectory are only picked up when recursive is true.
func TestBuilderDirectoryRecursion(t *testing.T) {
	const doc = `apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
`

	dir := t.TempDir()
	writeFile(t, dir, "top.yaml", fmt.Sprintf(doc, "top"))

	nested := filepath.Join(dir, "nested")
	require.NoError(t, os.Mkdir(nested, 0750))
	writeFile(t, nested, "deep.yaml", fmt.Sprintf(doc, "deep"))

	// A file whose extension is not in resource.FileExtensions is ignored when the
	// path is reached through a directory walk rather than named explicitly.
	writeFile(t, dir, "notes.txt", fmt.Sprintf(doc, "ignored"))

	names := func(t *testing.T, recursive bool) []string {
		t.Helper()
		objects, err := NewBuilder().Filename(recursive, dir).Do().Objects()
		require.NoError(t, err)

		got := make([]string, 0, len(objects))
		for _, o := range objects {
			obj, ok := o.(*map[string]any)
			require.True(t, ok, "unexpected object type %T", o)
			metadata, ok := (*obj)["metadata"].(map[string]any)
			require.True(t, ok, "unexpected metadata type %T", (*obj)["metadata"])
			got = append(got, metadata["name"].(string))
		}
		sort.Strings(got)
		return got
	}

	assert.Equal(t, []string{"top"}, names(t, false), "non-recursive must skip subdirectories")
	assert.Equal(t, []string{"deep", "top"}, names(t, true), "recursive must include subdirectories")
}

// TestExpandIfFilePattern covers the non-glob paths only. Whether a glob pattern
// reaches filepath.Glob is platform dependent: on Windows os.Stat rejects "*" as an
// invalid name rather than reporting it as missing, so the pattern is returned
// verbatim instead of being expanded.
func TestExpandIfFilePattern(t *testing.T) {
	dir := t.TempDir()
	existing := writeFile(t, dir, "a.yaml", "apiVersion: v1\n")

	tests := []struct {
		name         string
		pattern      string
		want         []string
		wantErr      bool
		wantErrMatch string
	}{
		{
			name:    "existing file is returned as is",
			pattern: existing,
			want:    []string{existing},
		},
		{
			name:         "missing file with no metacharacters is an error",
			pattern:      filepath.Join(dir, "absent.yaml"),
			wantErr:      true,
			wantErrMatch: "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandIfFilePattern(tt.pattern)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMatch)
				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
