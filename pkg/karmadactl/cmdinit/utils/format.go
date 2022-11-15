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

	"sigs.k8s.io/yaml"
)

const (
	// Query the IP address of the current host accessing the Internet
	getInternetIPUrl = "https://myexternalip.com/raw"
	// A split symbol that receives multiple values from a command flag
	separator      = ","
	labelSeparator = "="
	// MaxRespBodyLength is the max length of http response body
	MaxRespBodyLength = 1 << 20 // 1 MiB
)

// IsExist Determine whether the path exists
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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

	arr := strings.Split(ip, separator)
	for _, v := range arr {
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
	resp, err := http.Get(getInternetIPUrl)
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
	f, err := os.Create(filename)
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
