/*
Copyright 2021 The Karmada Authors.

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

package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDownloadFile test DownloadFile
func TestDownloadFile(t *testing.T) {
	const testFileName = "test.txt"
	testFileContent := []byte("test")

	serverTar := func() io.Reader {
		buf := bytes.NewBuffer(nil)
		gw := gzip.NewWriter(buf)
		defer func() {
			if err := gw.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		tw := tar.NewWriter(gw)
		defer func() {
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		hdr := &tar.Header{
			Name: testFileName,
			Mode: 0600,
			Size: int64(len(testFileContent)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(testFileContent); err != nil {
			t.Fatal(err)
		}
		return buf
	}()

	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		if _, err := io.Copy(rw, serverTar); err != nil {
			t.Fatal(err)
		}
	}))
	defer s.Close()

	tmpDir, err := os.MkdirTemp("", "karmada-test-download-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	downloadTar := filepath.Join(tmpDir, "test.tar.gz")
	err = DownloadFile(s.URL, downloadTar)
	if err != nil {
		t.Fatal(err)
	}
	err = DeCompress(downloadTar, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, testFileName))
	if err != nil {
		t.Fatal(err)
	}

	if want := testFileContent; !bytes.Equal(got, want) {
		t.Errorf("DeCompress() got %v, want %v", got, want)
	}
}

func TestListFiles(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		tempfiles []string
	}{
		{
			name:      "get files from path",
			path:      "temp-path" + randString(),
			tempfiles: []string{"tempfiles1" + randString(), "tempfiles2" + randString()},
		},
		{
			name:      "no files from path",
			path:      "temp-path" + randString(),
			tempfiles: []string{},
		},
	}
	for _, tt := range tests {
		err := os.Mkdir(tt.path, 0755)
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tt.path)

		var want []string
		for i := 0; i < len(tt.tempfiles); i++ {
			want = append(want, tt.path+"/"+tt.tempfiles[i])
			_, err = os.Create(tt.path + "/" + tt.tempfiles[i])
			if err != nil {
				t.Fatal(err)
			}
		}

		t.Run(tt.name, func(t *testing.T) {
			if got := ListFiles(tt.path + "/"); !reflect.DeepEqual(got, want) {
				t.Errorf("ListFiles() = %v, want %v", got, want)
			}
		})
	}
}
