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
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/karmada-io/karmada/pkg/util"
)


const (
	// A split symbol that receives multiple values from a command flag
	separator      = ","
	labelSeparator = "="
	// MaxRespBodyLength is the max length of http response body
	MaxRespBodyLength = 1 << 20 // 1 MiB
)

// getInternetIPUrl is the endpoint used to discover the host's public IP.
// It is a variable so that tests can substitute a local httptest.Server.
var getInternetIPUrl = "https://myexternalip.com/raw"

// InternetIPTimeout is the maximum time allowed to fetch the host's public IP.
// It is a variable so that tests can substitute a short value.
var InternetIPTimeout = 5 * time.Second

// StringToNetIP String To NetIP
func StringToNetIP(addr string) net.IP {
	if ip := net.ParseIP(addr); ip != nil {
		return ip
	}
	return net.ParseIP("127.0.0.1")
}

// FlagsIP Receive master external IP from command flags
func FlagsIP(ip string) []net.IP {
	var ips []net.IP

	arr := strings.SplitSeq(ip, separator)
	for v := range arr {
		ips = append(ips, StringToNetIP(v))
	}
	return ips
}

// FlagsDNS Receive master external DNS from command flags
func FlagsDNS(dns string) []string {
	return strings.Split(dns, separator)
}

// InternetIP Current host Internet IP.
func InternetIP() (net.IP, error) {
	client := &http.Client{Timeout: InternetIPTimeout}
	resp, err := client.Get(getInternetIPUrl)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(io.LimitReader(resp.Body, MaxRespBodyLength))
	if err != nil {
		return nil, err
	}

	return StringToNetIP(string(content)), nil
}

// FileToBytes File Conversion Bytes
func FileToBytes(path, name string) ([]byte, error) {
	filename := filepath.Join(path, name)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats, err := file.Stat()
	if err != nil {
		return nil, err
	}

	data := make([]byte, stats.Size())

	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// BytesToFile Bytes Conversion File
func BytesToFile(path, name string, data []byte) error {
	filename := filepath.Join(path, name)
	_, err := os.Stat(filename)
	if err == nil {
		return nil
	}

	// Create kubeconfig
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, util.DefaultFilePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// MapToString  labels to string
func MapToString(labels map[string]string) string {
	v := new(bytes.Buffer)
	for key, value := range labels {
		fmt.Fprintf(v, "%s=%s,", key, value)
	}
	return strings.TrimRight(v.String(), ",")
}

// StringToMap  string to label
func StringToMap(labels string) map[string]string {
	l := map[string]string{}
	slice := strings.Split(labels, labelSeparator)
	if len(slice) != 2 {
		return nil
	}

	l[slice[0]] = slice[1]
	return l
}

// StaticYamlToJSONByte  Static yaml file conversion JSON Byte
func StaticYamlToJSONByte(staticYaml string) []byte {
	jsonByte, err := yaml.YAMLToJSON([]byte(staticYaml))
	if err != nil {
		fmt.Println("Error convert string to json byte.")
		os.Exit(1)
	}
	return jsonByte
}
