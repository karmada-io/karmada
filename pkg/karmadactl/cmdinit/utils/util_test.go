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

	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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
