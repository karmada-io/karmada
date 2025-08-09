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

package util

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
	"github.com/karmada-io/karmada/pkg/util"
)

var (
	// ClientFactory creates a new Kubernetes clientset from the provided kubeconfig.
	ClientFactory = func(kubeconfig *rest.Config) (clientset.Interface, error) {
		return clientset.NewForConfig(kubeconfig)
	}

	// BuildClientFromSecretRefFactory constructs a Kubernetes clientset using a LocalSecretReference.
	BuildClientFromSecretRefFactory = func(client clientset.Interface, ref *operatorv1alpha1.LocalSecretReference) (clientset.Interface, error) {
		return BuildClientFromSecretRef(client, ref)
	}
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

// DownloadFile downloads files via URL, optionally using a proxy if provided.
func DownloadFile(url, filePath string, proxyConfig *operatorv1alpha1.ProxyConfig) error {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	if proxyConfig != nil {
		parsedProxyURL, err := urlpkg.ParseRequestURI(proxyConfig.ProxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(parsedProxyURL),
		}
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to download file. url: %s code: %v", url, resp.StatusCode)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, util.DefaultFilePerm)
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
			if err := os.Mkdir(targetPath+"/"+header.Name, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(targetPath+"/"+header.Name, os.O_CREATE|os.O_RDWR, util.DefaultFilePerm)
			if err != nil {
				return err
			}
			if err := ioCopyN(outFile, tr); err != nil {
				return err
			}
			outFile.Close()
		default:
			fmt.Printf("unknown type: %v in %s\n", header.Typeflag, header.Name)
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
	if err := filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			files = append(files, info)
		}
		return nil
	}); err != nil {
		fmt.Println(err)
	}

	return files
}

// FileExtInfo file info with absolute path
type FileExtInfo struct {
	os.FileInfo
	AbsPath string
}

// ListFileWithSuffix traverse directory files with suffix
func ListFileWithSuffix(path, suffix string) []FileExtInfo {
	files := []FileExtInfo{}
	if err := filepath.Walk(path, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() && strings.HasSuffix(path, suffix) {
			files = append(files, FileExtInfo{
				AbsPath:  path,
				FileInfo: info,
			})
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

// ReplaceYamlForRegs replace content of yaml file with Regexps
func ReplaceYamlForRegs(path string, replacements map[*regexp.Regexp]string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	src := string(data)
	for reg, dest := range replacements {
		src = reg.ReplaceAllString(src, dest)
	}

	return yaml.YAMLToJSON([]byte(src))
}

// ContainAllTasks checks if all tasks in the subset are present in the tasks slice.
// Returns an error if any subset task is not found; nil otherwise.
func ContainAllTasks(tasks, subset []workflow.Task) error {
	for _, subsetTask := range subset {
		found := false
		for _, task := range tasks {
			found = DeepEqualTasks(task, subsetTask) == nil
			if found {
				break
			}
		}
		if !found {
			return fmt.Errorf("subset task %v not found in tasks", subsetTask)
		}
	}
	return nil
}

// DeepEqualTasks checks if two workflow.Task instances are deeply equal.
// It returns an error if they differ, or nil if they are equal.
// The comparison includes the task name, RunSubTasks flag,
// and the length and contents of the Tasks slice.
// Function references and behavior are not compared; only the values
// of the specified fields are considered. Any differences are detailed
// in the returned error.
func DeepEqualTasks(t1, t2 workflow.Task) error {
	if t1.Name != t2.Name {
		return fmt.Errorf("expected t1 name %s, but got %s", t2.Name, t1.Name)
	}

	if t1.RunSubTasks != t2.RunSubTasks {
		return fmt.Errorf("expected t1 RunSubTasks flag %t, but got %t", t2.RunSubTasks, t1.RunSubTasks)
	}

	if len(t1.Tasks) != len(t2.Tasks) {
		return fmt.Errorf("expected t1 tasks length %d, but got %d", len(t2.Tasks), len(t1.Tasks))
	}

	for index := range t1.Tasks {
		err := DeepEqualTasks(t1.Tasks[index], t2.Tasks[index])
		if err != nil {
			return fmt.Errorf("unexpected error; tasks are not equal at index %d: %v", index, err)
		}
	}

	return nil
}

// ContainsAllValues checks if all values in the 'values' slice exist in the 'container' slice or array.
func ContainsAllValues(container interface{}, values interface{}) bool {
	// Ensure the provided container is a slice or array.
	vContainer := reflect.ValueOf(container)
	if vContainer.Kind() != reflect.Slice && vContainer.Kind() != reflect.Array {
		return false
	}

	// Ensure the provided values are a slice or array.
	vValues := reflect.ValueOf(values)
	if vValues.Kind() != reflect.Slice && vValues.Kind() != reflect.Array {
		return false
	}

	// Iterate over the 'values' and ensure each value exists in the container.
	for i := 0; i < vValues.Len(); i++ {
		value := vValues.Index(i).Interface()
		found := false
		// Check if this value exists in the container.
		for j := 0; j < vContainer.Len(); j++ {
			if reflect.DeepEqual(vContainer.Index(j).Interface(), value) {
				found = true
				break
			}
		}
		// If any value is not found, return false.
		if !found {
			return false
		}
	}
	// If all values were found, return true.
	return true
}
