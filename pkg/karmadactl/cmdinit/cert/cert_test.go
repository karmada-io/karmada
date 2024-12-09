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

package cert

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	TestCertsTmp        = "./test-certs-tmp-without-ca-certificate"        //nolint
	TestCertsTmpWithArg = "./test-certs-tmp-with-ca-certificate"           //nolint
	TestCaCertPath      = "./test-certs-tmp-without-ca-certificate/ca.crt" //nolint
	TestCaKeyPath       = "./test-certs-tmp-without-ca-certificate/ca.key" //nolint
)

var certFiles = []string{
	"apiserver.crt", "apiserver.key",
	"ca.crt", "ca.key",
	"etcd-ca.crt", "etcd-ca.key",
	"etcd-client.crt", "etcd-client.key",
	"etcd-server.crt", "etcd-server.key",
	"front-proxy-ca.crt", "front-proxy-ca.key",
	"front-proxy-client.crt", "front-proxy-client.key",
	"karmada.crt", "karmada.key",
}

func TestGenCerts(t *testing.T) {
	defer os.RemoveAll(TestCertsTmp)
	defer os.RemoveAll(TestCertsTmpWithArg)

	notAfter := time.Now().Add(Duration365d * 10).UTC()
	namespace := "kube-karmada"
	flagsExternalIP := ""
	masterIP := "192.168.1.1,192.168.1.2"

	var etcdServerCertDNS = []string{
		"localhost",
	}

	for i := 0; i < 3; i++ {
		etcdServerCertDNS = append(etcdServerCertDNS, fmt.Sprintf("%s-%v.%s.%s.svc.cluster.local", "etcd", i, "etcd", namespace))
	}

	etcdServerAltNames := certutil.AltNames{
		DNSNames: etcdServerCertDNS,
		IPs:      []net.IP{utils.StringToNetIP("127.0.0.1")},
	}
	etcdServerCertConfig := NewCertConfig("karmada-etcd-server", []string{}, etcdServerAltNames, &notAfter)

	etcdClientCertCfg := NewCertConfig("karmada-etcd-client", []string{}, certutil.AltNames{}, &notAfter)

	var karmadaDNS = []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"karmada-apiserver",
		names.KarmadaWebhookComponentName,
		fmt.Sprintf("%s.%s.svc.cluster.local", "karmada-apiserver", namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", names.KarmadaWebhookComponentName, namespace),
		fmt.Sprintf("*.%s.svc.cluster.local", namespace),
		fmt.Sprintf("*.%s.svc", namespace),
	}
	if hostName, err := os.Hostname(); err != nil {
		klog.Warningf("Failed to get the current hostname, error message: %s.", err)
	} else {
		karmadaDNS = append(karmadaDNS, hostName)
	}

	ips := utils.FlagsIP(flagsExternalIP)
	ips = append(ips, utils.FlagsIP(masterIP)...)

	internetIP, err := utils.InternetIP()
	if err != nil {
		klog.Warningf("Failed to obtain internet IP, error message: %s.", err)
	} else {
		ips = append(ips, internetIP)
	}

	ips = append(
		ips,
		utils.StringToNetIP("127.0.0.1"),
		utils.StringToNetIP("10.254.0.1"),
	)

	fmt.Println("karmada certificate ip ", ips)

	karmadaAltNames := certutil.AltNames{
		DNSNames: karmadaDNS,
		IPs:      ips,
	}

	karmadaCertCfg := NewCertConfig("system:admin", []string{"system:masters"}, karmadaAltNames, &notAfter)
	apiserverCertCfg := NewCertConfig("karmada-apiserver", []string{""}, karmadaAltNames, &notAfter)
	frontProxyClientCertCfg := NewCertConfig("front-proxy-client", []string{}, certutil.AltNames{}, &notAfter)

	if err := GenCerts(TestCertsTmp, "", "", etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, apiserverCertCfg, frontProxyClientCertCfg); err != nil {
		t.Fatal(err)
	}
	if err := checkCertFiles(TestCertsTmp, certFiles); err != nil {
		t.Fatal(err)
	} else {
		klog.Infof("All certificate files are present without CA certificates address parameter exists")
	}

	if err := GenCerts(TestCertsTmpWithArg, TestCaCertPath, TestCaKeyPath, etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, apiserverCertCfg, frontProxyClientCertCfg); err != nil {
		t.Fatal(err)
	}
	if err := checkCertFiles(TestCertsTmpWithArg, certFiles); err != nil {
		t.Fatal(err)
	} else {
		klog.Infof("All certificate files are present with CA certificates address parameter exists")
	}

	if ok, err := compareCertFilesInDirs(TestCertsTmp, TestCertsTmpWithArg, "ca.crt"); !ok || err != nil {
		t.Fatal(err)
	} else {
		klog.Infof("The certificate files in the two directories are the same")
	}
}

func checkCertFiles(path string, files []string) error {
	for _, file := range files {
		filePath := filepath.Join(path, file)
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("cert file not found: %s,error message: %s", filePath, err.Error())
		}
	}
	return nil
}

// compareFiles compares two files to see if they are the same
func compareFiles(file1, file2 string) (bool, error) {
	f1, err := os.Open(file1)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %v", file1, err)
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %v", file2, err)
	}
	defer f2.Close()

	hash1 := sha256.New()
	hash2 := sha256.New()

	if _, err := io.Copy(hash1, f1); err != nil {
		return false, fmt.Errorf("failed to read file %s: %v", file1, err)
	}
	if _, err := io.Copy(hash2, f2); err != nil {
		return false, fmt.Errorf("failed to read file %s: %v", file2, err)
	}

	return string(hash1.Sum(nil)) == string(hash2.Sum(nil)), nil
}

// compareCertFilesInDirs compares specific files in two directories to check if they are the same
func compareCertFilesInDirs(dir1, dir2, filename string) (bool, error) {
	file1 := filepath.Join(dir1, filename)
	file2 := filepath.Join(dir2, filename)
	return compareFiles(file1, file2)
}
