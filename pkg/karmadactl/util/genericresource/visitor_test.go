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
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIgnoreFile(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       bool
	}{
		{
			// No extension filter means every file is accepted.
			name: "no extensions accepts everything",
			path: "manifest.txt",
			want: false,
		},
		{
			name:       "matching extension is kept",
			path:       "manifest.yaml",
			extensions: []string{".json", ".yaml", ".yml"},
			want:       false,
		},
		{
			name:       "unmatched extension is ignored",
			path:       "README.md",
			extensions: []string{".json", ".yaml", ".yml"},
			want:       true,
		},
		{
			// filepath.Ext returns "" here, which is not in the list.
			name:       "file without extension is ignored",
			path:       "Makefile",
			extensions: []string{".json", ".yaml", ".yml"},
			want:       true,
		},
		{
			name:       "extension match is case sensitive",
			path:       "manifest.YAML",
			extensions: []string{".json", ".yaml", ".yml"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ignoreFile(tt.path, tt.extensions))
		})
	}
}

// countingBody reports whether the reader was closed, so the retry loop can be
// checked for leaking response bodies.
type countingBody struct {
	io.Reader
	closed bool
}

func (c *countingBody) Close() error {
	c.closed = true
	return nil
}

// stubResponse is one canned reply from the stubbed httpget.
type stubResponse struct {
	code int
	err  error
}

func TestReadHTTPWithRetries(t *testing.T) {
	tests := []struct {
		name         string
		responses    []stubResponse
		attempts     int
		wantCalls    int
		wantErr      bool
		wantErrMatch string
	}{
		{
			name:         "non positive attempts is rejected without calling get",
			attempts:     0,
			wantCalls:    0,
			wantErr:      true,
			wantErrMatch: "http attempts must be greater than 0",
		},
		{
			name:      "success on the first attempt",
			responses: []stubResponse{{code: http.StatusOK}},
			attempts:  3,
			wantCalls: 1,
		},
		{
			name: "server errors are retried until one succeeds",
			responses: []stubResponse{
				{code: http.StatusInternalServerError},
				{code: http.StatusBadGateway},
				{code: http.StatusOK},
			},
			attempts:  3,
			wantCalls: 3,
		},
		{
			// 4xx is a client error, so retrying cannot help.
			name: "client errors are not retried",
			responses: []stubResponse{
				{code: http.StatusNotFound},
				{code: http.StatusOK},
			},
			attempts:     3,
			wantCalls:    1,
			wantErr:      true,
			wantErrMatch: "status code=404",
		},
		{
			name: "transport errors are retried",
			responses: []stubResponse{
				{err: errors.New("connection refused")},
				{code: http.StatusOK},
			},
			attempts:  2,
			wantCalls: 2,
		},
		{
			name: "gives up after the last attempt",
			responses: []stubResponse{
				{code: http.StatusInternalServerError},
				{code: http.StatusInternalServerError},
			},
			attempts:     2,
			wantCalls:    2,
			wantErr:      true,
			wantErrMatch: "status code=500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			var bodies []*countingBody

			get := func(string) (int, string, io.ReadCloser, error) {
				i := calls
				calls++
				require.Less(t, i, len(tt.responses), "get called more often than the test has responses")
				r := tt.responses[i]
				if r.err != nil {
					return 0, "", nil, r.err
				}
				body := &countingBody{Reader: strings.NewReader("payload")}
				bodies = append(bodies, body)
				return r.code, http.StatusText(r.code), body, nil
			}

			body, err := readHTTPWithRetries(get, time.Nanosecond, "http://example.com/manifest.yaml", tt.attempts)

			assert.Equal(t, tt.wantCalls, calls, "unexpected number of attempts")

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMatch != "" {
					assert.Contains(t, err.Error(), tt.wantErrMatch)
				}
				assert.Nil(t, body)
				// A body belonging to a failed attempt must not be leaked.
				for i, b := range bodies {
					assert.True(t, b.closed, "body from attempt %d was not closed", i)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, body)
			got, readErr := io.ReadAll(body)
			require.NoError(t, readErr)
			assert.Equal(t, "payload", string(got))
			require.NoError(t, body.Close())

			// Only the body that was handed back stays open; earlier ones are closed.
			for i, b := range bodies[:len(bodies)-1] {
				assert.True(t, b.closed, "body from attempt %d was not closed", i)
			}
		})
	}
}

func TestStreamVisitorVisit(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantNames    []string
		wantErr      bool
		wantErrMatch string
	}{
		{
			name:      "single document",
			input:     "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: first\n",
			wantNames: []string{"first"},
		},
		{
			name: "multiple documents are visited in order",
			input: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: first\n" +
				"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: second\n",
			wantNames: []string{"first", "second"},
		},
		{
			// Documents that carry nothing are skipped rather than surfaced.
			name: "empty documents are skipped",
			input: "---\n\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: only\n" +
				"---\n\n",
			wantNames: []string{"only"},
		},
		{
			name:      "explicit null document is skipped",
			input:     "null\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: only\n",
			wantNames: []string{"only"},
		},
		{
			name:         "malformed document reports the source",
			input:        "\tbad: indentation\n",
			wantErr:      true,
			wantErrMatch: "error parsing testsource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewStreamVisitor(
				strings.NewReader(tt.input),
				&mapper{newFunc: defaultNewFunc},
				"testsource",
				nil,
			)

			var names []string
			err := v.Visit(func(info *Info, visitErr error) error {
				if visitErr != nil {
					return visitErr
				}
				obj, ok := info.Object.(*map[string]any)
				require.True(t, ok, "unexpected object type %T", info.Object)
				metadata, ok := (*obj)["metadata"].(map[string]any)
				require.True(t, ok, "object has no metadata")
				name, ok := metadata["name"].(string)
				require.True(t, ok, "object has no metadata.name")
				names = append(names, name)
				assert.Equal(t, "testsource", info.Source)
				return nil
			})

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMatch)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNames, names)
		})
	}
}
