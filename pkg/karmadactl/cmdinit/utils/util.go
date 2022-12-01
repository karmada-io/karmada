package utils

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader Download progress
type Downloader struct {
	io.Reader
	Total   int64
	Current int64
}

// Read Implementation of Downloader
func (d *Downloader) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	if err != nil {
		if err != io.EOF {
			return
		}
		fmt.Println("\nDownload complete.")
		return
	}

	d.Current += int64(n)
	fmt.Printf("\rDownloading...[ %.2f%% ]", float64(d.Current*10000/d.Total)/100)
	return
}

// DownloadFile Download files via URL
func DownloadFile(url, filePath string) error {
	httpClient := http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed download file. url: %s code: %v", url, resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	downloader := &Downloader{
		Reader: resp.Body,
		Total:  resp.ContentLength,
	}

	if _, err := io.Copy(file, downloader); err != nil {
		return err
	}

	return nil
}

// DeCompress decompress tar.gz
func DeCompress(file, targetPath string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("new reader failed. %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(targetPath+"/"+header.Name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(targetPath + "/" + header.Name)
			if err != nil {
				return err
			}
			if err := ioCopyN(outFile, tr); err != nil {
				return err
			}
			outFile.Close()
		default:
			fmt.Printf("uknown type: %v in %s\n", header.Typeflag, header.Name)
		}
	}
	return nil
}

// ioCopyN fix Potential DoS vulnerability via decompression bomb.
func ioCopyN(outFile *os.File, tr *tar.Reader) error {
	for {
		if _, err := io.CopyN(outFile, tr, 1024); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	return nil
}

// ListFiles traverse directory files
func ListFiles(path string) []string {
	var files []string
	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		fmt.Println(err)
	}
	return files
}
