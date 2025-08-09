/*
Copyright 2023 The Karmada Authors.

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
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

// mockReader is a simple io.Reader that returns an error after being called.
type mockReader struct {
	data []byte
	err  error
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.data == nil {
		return 0, m.err
	}
	n = copy(p, m.data)
	m.data = m.data[n:]
	return n, m.err
}

func TestRead(t *testing.T) {
	tests := []struct {
		name       string
		downloader *Downloader
		data       string
		prep       func(downloader *Downloader, data string) error
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Read_FailedToReadFromDataSource_ReadFailed",
			downloader: &Downloader{
				Reader: &mockReader{
					err: errors.New("unexpected read error"),
				},
			},
			prep: func(*Downloader, string) error {
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected read error",
		},
		{
			name: "Read_FailedToReadWithEOF_ReadFailed",
			downloader: &Downloader{
				Reader: &mockReader{
					err: io.EOF,
				},
			},
			prep: func(*Downloader, string) error {
				return nil
			},
			wantErr: true,
			errMsg:  "EOF",
		},
		{
			name: "Read_FromValidDataSource_ReadSucceeded",
			downloader: &Downloader{
				Current: 3,
				Total:   10,
			},
			data: "test data",
			prep: func(downloader *Downloader, data string) error {
				downloader.Reader = &mockReader{data: []byte(data)}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.downloader, test.data); err != nil {
				t.Fatalf("failed to prep before reading the data, got: %v", err)
			}
			buffer := test.downloader.Reader.(*mockReader).data
			_, err := test.downloader.Read(buffer)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if string(buffer) != test.data {
				t.Errorf("expected read buffer data to be %s, but got %s", test.data, string(buffer))
			}
		})
	}
}

func TestDownloadFile(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		proxyConfig *v1alpha1.ProxyConfig
		filePath    string
		verify      func(filePath string) error
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "DownloadFile_UrlIsNotFound_FailedToGetResponse",
			url:         "not-found-url",
			proxyConfig: nil,
			verify:      func(string) error { return nil },
			wantErr:     true,
			errMsg:      "failed to get url not-found-url, url is not found",
		},
		{
			name: "DownloadFile_ServiceIsUnavailable_FailedToReachTheService",
			url:  "https://www.example.com/test-file",
			proxyConfig: &v1alpha1.ProxyConfig{
				ProxyURL: "http://www.dummy-proxy.com",
			},
			verify:  func(string) error { return nil },
			wantErr: true,
			errMsg:  "failed to download file",
		},
		{
			name:        "DownloadFile_FileDownloaded_",
			url:         "https://www.example.com",
			proxyConfig: nil,
			filePath:    filepath.Join(os.TempDir(), "temp-download-file.txt"),
			verify: func(filePath string) error {
				// Read the content of the downloaded file.
				content, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed   to read file: %w", err)
				}

				// Verify that content has been downloaded and saved to the file
				if len(content) == 0 {
					return fmt.Errorf("file is empty: %s", filePath)
				}

				if err := os.Remove(filePath); err != nil {
					return fmt.Errorf("failed to clean up %s", filePath)
				}

				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := DownloadFile(test.url, test.filePath, test.proxyConfig)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(test.filePath); err != nil {
				t.Errorf("failed to verify the actual of download of file: %v", err)
			}
		})
	}
}

func TestUnpack(t *testing.T) {
	tests := []struct {
		name        string
		tarFile     string
		regularFile string
		targetPath  *string
		prep        func(tarFile, regularFile string, targetPath *string) error
		verify      func(regularFile string, targetPath string) error
		wantErr     bool
		errMsg      string
	}{
		{
			name:       "Unpack_InvalidGzipFileHeader_InvalidHeader",
			tarFile:    "invalid.tar.gz",
			targetPath: ptr.To(""),
			prep: func(tarFile, _ string, targetPath *string) error {
				var err error
				*targetPath, err = os.MkdirTemp("", "test-unpack-*")
				if err != nil {
					return err
				}
				f, err := os.Create(filepath.Join(*targetPath, tarFile))
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = f.WriteString("Invalid gzip content")
				return err
			},
			verify: func(_, targetPath string) error {
				return os.RemoveAll(targetPath)
			},
			wantErr: true,
			errMsg:  gzip.ErrHeader.Error(),
		},
		{
			name:        "Unpack_ValidTarGzipped_UnpackedSuccessfully",
			tarFile:     "valid.tar.gz",
			regularFile: "test-file.txt",
			targetPath:  ptr.To(""),
			prep:        verifyValidTarGzipped,
			verify: func(regularFile string, targetPath string) error {
				fileExpected := filepath.Join(targetPath, "test", regularFile)
				_, err := os.Stat(fileExpected)
				if err != nil {
					return fmt.Errorf("failed to find the file %s, got error: %v", fileExpected, err)
				}
				return os.RemoveAll(targetPath)
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.tarFile, test.regularFile, test.targetPath); err != nil {
				t.Fatalf("failed to prep before unpacking the tar file: %v", err)
			}
			err := Unpack(filepath.Join(*test.targetPath, test.tarFile), *test.targetPath)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.regularFile, *test.targetPath); err != nil {
				t.Errorf("failed to verify unpacking process, got: %v", err)
			}
		})
	}
}

func TestListFileWithSuffix(t *testing.T) {
	suffix := ".yaml"
	files := ListFileWithSuffix(".", suffix)
	want := 2
	got := len(files)
	if want != got {
		t.Errorf("Expected %d ,but got :%d", want, got)
	}
	for _, f := range files {
		if !strings.HasSuffix(f.AbsPath, suffix) {
			t.Errorf("Want suffix with %s , but not exist, path is:%s", suffix, f.AbsPath)
		}
	}
}

// verifyValidTarGzipped creates a tar.gz file in a temporary directory.
// The archive contains a "test" directory and a file with the specified name,
// containing the message "Hello, World!".
func verifyValidTarGzipped(tarFile, regularFile string, targetPath *string) error {
	var err error
	*targetPath, err = os.MkdirTemp("", "test-unpack-*")
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(*targetPath, tarFile))
	if err != nil {
		return err
	}
	defer f.Close()

	// Create a gzip writer.
	gw := gzip.NewWriter(f)
	defer gw.Close()

	// Create a tar writer.
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add a directory to the tar.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "test" + string(filepath.Separator),
		Typeflag: tar.TypeDir,
		Mode:     0755,
	}); err != nil {
		return err
	}

	// Add a file to the tar.
	message := "Hello, World!"
	if err := tw.WriteHeader(&tar.Header{
		Name:     filepath.Join("test", regularFile),
		Mode:     0644,
		Size:     int64(len(message)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(message)); err != nil {
		return err
	}

	return nil
}

func TestReplaceYamlForRegs(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		replacements    map[*regexp.Regexp]string
		expectedContent string
	}{
		{
			name: "simple replacement",
			content: `
        url: "https://{{name}}.{{namespace}}.svc:443/convert"
        caBundle: "{{caBundle}}"
`,
			replacements: map[*regexp.Regexp]string{
				regexp.MustCompile("{{caBundle}}"):  "testCaBundle",
				regexp.MustCompile("{{name}}"):      "testName",
				regexp.MustCompile("{{namespace}}"): "testNamespace",
			},
			expectedContent: `
        url: "https://testName.testNamespace.svc:443/convert"
        caBundle: "testCaBundle"
`,
		},
		{
			name: "partial replacement",
			content: `
        url: "https://{{name}}.{{namespace}}.svc:443/convert"
        caBundle: "{{caBundle}}"
`,
			replacements: map[*regexp.Regexp]string{
				regexp.MustCompile("{{caBundle}}"):  "testCaBundle",
				regexp.MustCompile("{{namespace}}"): "testNamespace",
			},
			expectedContent: `
        url: "https://{{name}}.testNamespace.svc:443/convert"
        caBundle: "testCaBundle"
`,
		},
		{
			name: "redundant replacement",
			content: `
        url: "https://{{name}}.{{namespace}}.svc:443/convert"
        caBundle: "{{caBundle}}"
`,
			replacements: map[*regexp.Regexp]string{
				regexp.MustCompile("{{caBundle}}"):  "testCaBundle",
				regexp.MustCompile("{{name}}"):      "testName",
				regexp.MustCompile("{{namespace}}"): "testNamespace",
				regexp.MustCompile("{{foo}}"):       "foo",
			},
			expectedContent: `
        url: "https://testName.testNamespace.svc:443/convert"
        caBundle: "testCaBundle"
`,
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

			result, err := ReplaceYamlForRegs(tmpFile.Name(), tt.replacements)
			expectedJSON, expectedErr := yaml.YAMLToJSON([]byte(tt.expectedContent))

			assert.Equal(t, result, expectedJSON)
			assert.Equal(t, err, expectedErr)
		})
	}
}
