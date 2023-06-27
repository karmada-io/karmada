package util

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
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
		klog.Info("\nDownload complete.")
		return
	}

	d.Current += int64(n)
	klog.Infof("\rDownloading...[ %.2f%% ]", float64(d.Current*10000/d.Total)/100)
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

// Unpack unpack a given file to target path
func Unpack(file, targetPath string) error {
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
func ListFiles(path string) []os.FileInfo {
	var files []os.FileInfo
	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, info)
		}
		return nil
	}); err != nil {
		fmt.Println(err)
	}

	return files
}

// PathExists check whether the path is exist
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// ReadYamlFile ready file given path with yaml format
func ReadYamlFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return yaml.YAMLToJSON(data)
}

// RelpaceYamlForReg replace content of yaml file with a Regexp
func RelpaceYamlForReg(path, destResource string, reg *regexp.Regexp) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	repl := reg.ReplaceAllString(string(data), destResource)
	return yaml.YAMLToJSON([]byte(repl))
}
