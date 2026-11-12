/*
Copyright 2022 The Karmada Authors.

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

package kubernetes

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"

	certpkg "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/cert"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/config"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	globaloptions "github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

func Test_initializeDirectory(t *testing.T) {
	tests := []struct {
		name                string
		createPathInAdvance bool
		wantErr             bool
	}{
		{
			name:                "Test when there is no dir exists",
			createPathInAdvance: false,
			wantErr:             false,
		},
		{
			name:                "Test when there is dir exists",
			createPathInAdvance: true,
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createPathInAdvance {
				if err := os.MkdirAll("tmp", os.FileMode(0700)); err != nil {
					t.Errorf("create test directory failed in advance:%v", err)
				}
			}
			if err := initializeDirectory("tmp"); (err != nil) != tt.wantErr {
				t.Errorf("initializeDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := os.RemoveAll("tmp"); err != nil {
				t.Errorf("clean up test directory failed after ut case:%s, %v", tt.name, err)
			}
		})
	}
}

func TestCommandInitOption_Validate(t *testing.T) {
	tests := []struct {
		name     string
		opt      CommandInitOption
		wantErr  bool
		errorMsg string
	}{
		{
			name: "Invalid KarmadaAPIServerAdvertiseAddress",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "111",
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when KarmadaAPIServerAdvertiseAddress is wrong",
		},
		{
			name: "Invalid cert-external-ip",
			opt: CommandInitOption{
				ExternalIP:      "192.168.1.300",
				EtcdStorageMode: etcdStorageModeEmptyDir,
				ImagePullPolicy: string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when cert-external-ip is not a valid IP",
		},
		{
			name: "cert-external-ip with spaces is rejected",
			opt: CommandInitOption{
				// A space after the comma is not tolerated by net.ParseIP, which is
				// how the value is consumed during cert generation, so it must be
				// rejected here rather than silently becoming 127.0.0.1.
				ExternalIP:      "192.0.2.1, 192.0.2.2",
				EtcdStorageMode: etcdStorageModeEmptyDir,
				ImagePullPolicy: string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when cert-external-ip contains a space",
		},
		{
			name: "Valid cert-external-ip",
			opt: CommandInitOption{
				ExternalIP:      "192.0.2.1,192.0.2.2",
				EtcdStorageMode: etcdStorageModeEmptyDir,
				ImagePullPolicy: string(corev1.PullIfNotPresent),
			},
			wantErr:  false,
			errorMsg: "CommandInitOption.Validate() returns err when cert-external-ip is a valid IP list",
		},
		{
			name: "Empty EtcdHostDataPath",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeHostPath,
				EtcdHostDataPath:                 "",
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdHostDataPath is empty",
		},
		{
			name: "Invalid EtcdNodeSelectorLabels",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeHostPath,
				EtcdHostDataPath:                 "/data",
				EtcdNodeSelectorLabels:           "key",
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdNodeSelectorLabels is %v",
		},
		{
			name: "Invalid EtcdReplicas",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeHostPath,
				EtcdHostDataPath:                 "/data",
				EtcdNodeSelectorLabels:           "key=value",
				EtcdReplicas:                     2,
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdReplicas is %v",
		},
		{
			name: "Empty StorageClassesName",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModePVC,
				EtcdHostDataPath:                 "/data",
				EtcdNodeSelectorLabels:           "key=value",
				EtcdReplicas:                     1,
				StorageClassesName:               "",
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when StorageClassesName is empty",
		},
		{
			name: "Invalid EtcdStorageMode",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  "unknown",
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdStorageMode is unknown",
		},
		{
			name: "Valid EtcdStorageMode",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeEmptyDir,
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  false,
			errorMsg: "CommandInitOption.Validate() returns err when EtcdStorageMode is emptyDir",
		},
		{
			name: "Valid EtcdStorageMode",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  "",
				ImagePullPolicy:                  string(corev1.PullIfNotPresent),
			},
			wantErr:  false,
			errorMsg: "CommandInitOption.Validate() returns err when EtcdStorageMode is empty",
		},
		{
			name: "Invalid ImagePullPolicy",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  "",
				ImagePullPolicy:                  "NotExistImagePullPolicy",
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() returns err when invalid ImagePullPolicy",
		},
		{
			name: "Invalid CertMode",
			opt: CommandInitOption{
				CertMode:        "unknown",
				EtcdStorageMode: "",
				ImagePullPolicy: string(corev1.PullIfNotPresent),
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when cert-mode is invalid",
		},
		{
			name: "Valid rotate CertMode",
			opt: CommandInitOption{
				CertMode:        CertModeRotate,
				EtcdStorageMode: "",
				ImagePullPolicy: string(corev1.PullIfNotPresent),
			},
			wantErr:  false,
			errorMsg: "CommandInitOption.Validate() returns err when cert-mode is rotate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.opt.Validate("parentCommand"); (err != nil) != tt.wantErr {
				t.Errorf("%s err = %v, want %v", tt.name, err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestCommandInitOption_completeRotateSkipsInstallOnlyChecks(t *testing.T) {
	clientset := fake.NewClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-1",
				Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Address: "192.0.2.10"}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-node-port-service",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{{
					NodePort: 32443,
				}},
			},
		},
	)
	opt := &CommandInitOption{
		CertMode:                         CertModeRotate,
		KubeClientSet:                    clientset,
		KarmadaAPIServerNodePort:         32443,
		EtcdStorageMode:                  etcdStorageModeHostPath,
		KarmadaAPIServerAdvertiseAddress: "",
	}

	err := opt.completeRotate()
	assert.NoError(t, err)

	for _, action := range clientset.Actions() {
		assert.NotEqual(t, "services", action.GetResource().Resource)
		assert.NotEqual(t, "patch", action.GetVerb())
	}
}

func TestCommandInitOption_genCerts(t *testing.T) {
	tmpPath := "/tmp/pki"
	defer os.RemoveAll(tmpPath)
	opt := &CommandInitOption{
		CertValidity:       365 * 24 * time.Hour,
		EtcdReplicas:       3,
		Namespace:          "default",
		ExternalDNS:        "example.com",
		ExternalIP:         "1.2.3.4",
		KarmadaAPIServerIP: []net.IP{utils.StringToNetIP("1.2.3.5")},
		KarmadaPkiPath:     tmpPath,
	}
	// Call the function to generate the certificates
	err := opt.genCerts()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCommandInitOption_runCertRotateUpdatesSecretsOnly(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             3,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "host.example",
		ExternalDNS:              "api.example.test",
		ExternalIP:               "198.51.100.10",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KarmadaDataPath:          t.TempDir(),
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedKarmadaSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, oldCertData[certFileName(globaloptions.CaCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(globaloptions.CaCertAndKeyName)])
	assert.Equal(t, oldCertData[keyFileName(globaloptions.CaCertAndKeyName)], updatedKarmadaSecret.StringData[keyFileName(globaloptions.CaCertAndKeyName)])
	assert.NotEqual(t, oldCertData[certFileName(options.KarmadaCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(options.KarmadaCertAndKeyName)])
	assert.True(t, bytes.Equal([]byte(oldCertData[keyFileName(options.KarmadaCertAndKeyName)]), []byte(updatedKarmadaSecret.StringData[keyFileName(options.KarmadaCertAndKeyName)])), "karmada private key must be preserved")

	rotatedKarmadaCert, err := certpkg.LoadCertificatePEM([]byte(updatedKarmadaSecret.StringData[certFileName(options.KarmadaCertAndKeyName)]))
	assert.NoError(t, err)
	rootCA, err := certpkg.LoadCertificatePEM([]byte(updatedKarmadaSecret.StringData[certFileName(globaloptions.CaCertAndKeyName)]))
	assert.NoError(t, err)
	assert.NoError(t, rotatedKarmadaCert.CheckSignatureFrom(rootCA))
	assert.Contains(t, rotatedKarmadaCert.DNSNames, "api.example.test")
	assert.True(t, slices.ContainsFunc(rotatedKarmadaCert.IPAddresses, func(ip net.IP) bool {
		return ip.Equal(net.ParseIP("198.51.100.10"))
	}))
	_, _, err = certpkg.LoadCertAndKeyPEM(
		[]byte(updatedKarmadaSecret.StringData[certFileName(options.KarmadaCertAndKeyName)]),
		[]byte(oldCertData[keyFileName(options.KarmadaCertAndKeyName)]),
	)
	assert.NoError(t, err)
	rotatedEtcdServerCert, err := certpkg.LoadCertificatePEM([]byte(updatedKarmadaSecret.StringData[certFileName(options.EtcdServerCertAndKeyName)]))
	assert.NoError(t, err)
	for replica := range 3 {
		assert.Contains(t, rotatedEtcdServerCert.DNSNames, fmt.Sprintf("%s-%d.%s.%s.svc.host.example",
			etcdStatefulSetAndServiceName, replica, etcdStatefulSetAndServiceName, namespace))
	}

	updatedWebhookSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), webhookCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, updatedKarmadaSecret.StringData[certFileName(options.KarmadaCertAndKeyName)], updatedWebhookSecret.StringData["tls.crt"])
	assert.Equal(t, oldCertData[keyFileName(options.KarmadaCertAndKeyName)], updatedWebhookSecret.StringData["tls.key"])

	assertNoInstallResourceActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateConvergesAfterSecretUpdateTimeout(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	failedOnce := false
	clientset.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		secret := action.(k8stesting.UpdateAction).GetObject().(*corev1.Secret)
		if !failedOnce && secret.Name == webhookCertsName {
			failedOnce = true
			return true, nil, apierrors.NewTimeoutError("forced Secret update timeout", 1)
		}
		return false, nil, nil
	})
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	assert.True(t, apierrors.IsTimeout(err))
	assert.True(t, failedOnce)

	partiallyUpdatedSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotEqual(t, oldCertData[certFileName(options.KarmadaCertAndKeyName)], partiallyUpdatedSecret.StringData[certFileName(options.KarmadaCertAndKeyName)])

	assert.NoError(t, opt.runCertRotate())
	expectedSecrets, err := opt.certSecretSpecs()
	assert.NoError(t, err)
	for _, expectedSecret := range expectedSecrets {
		actualSecret, getErr := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), expectedSecret.Name, metav1.GetOptions{})
		assert.NoError(t, getErr)
		assert.Equal(t, expectedSecret.StringData, actualSecret.StringData)
	}
}

func TestCommandInitOption_runCertRotateRenewsExpiringLeafCertificates(t *testing.T) {
	namespace := "karmada-system"
	now := time.Now().UTC()
	oldCertData := buildTestCertAndKeyDataWithLeafNotAfter(t, now.Add(time.Minute))
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	oldKarmadaCert := loadTestCertFromData(t, oldCertData, options.KarmadaCertAndKeyName)
	oldEtcdServerCert := loadTestCertFromData(t, oldCertData, options.EtcdServerCertAndKeyName)
	assert.WithinDuration(t, now.Add(time.Minute), oldKarmadaCert.NotAfter, 10*time.Second)
	assert.WithinDuration(t, now.Add(time.Minute), oldEtcdServerCert.NotAfter, 10*time.Second)

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedKarmadaSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	updatedEtcdSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), etcdCertName, metav1.GetOptions{})
	assert.NoError(t, err)
	rootCA := loadTestCertFromSecret(t, updatedKarmadaSecret, globaloptions.CaCertAndKeyName)
	etcdCA := loadTestCertFromSecret(t, updatedEtcdSecret, options.EtcdCaCertAndKeyName)
	rotatedKarmadaCert := loadTestCertFromSecret(t, updatedKarmadaSecret, options.KarmadaCertAndKeyName)
	rotatedEtcdServerCert := loadTestCertFromSecret(t, updatedEtcdSecret, options.EtcdServerCertAndKeyName)

	assert.Equal(t, oldCertData[certFileName(globaloptions.CaCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(globaloptions.CaCertAndKeyName)])
	assert.Equal(t, oldCertData[certFileName(options.EtcdCaCertAndKeyName)], updatedEtcdSecret.StringData[certFileName(options.EtcdCaCertAndKeyName)])
	assert.NoError(t, rotatedKarmadaCert.CheckSignatureFrom(rootCA))
	assert.NoError(t, rotatedEtcdServerCert.CheckSignatureFrom(etcdCA))
	assert.True(t, rotatedKarmadaCert.NotAfter.After(now.Add(300*24*time.Hour)))
	assert.True(t, rotatedEtcdServerCert.NotAfter.After(now.Add(300*24*time.Hour)))
	assert.True(t, rotatedKarmadaCert.NotAfter.After(oldKarmadaCert.NotAfter.Add(300*24*time.Hour)))
	assert.True(t, rotatedEtcdServerCert.NotAfter.After(oldEtcdServerCert.NotAfter.Add(300*24*time.Hour)))
}

func TestCommandInitOption_runCertRotatePreservesExistingServingCertificateSANs(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	replaceTestLeafCertAndKeyData(t, oldCertData, options.KarmadaCertAndKeyName, globaloptions.CaCertAndKeyName, certutil.AltNames{
		DNSNames: []string{"old-webhook.example.test"},
		IPs:      []net.IP{net.ParseIP("203.0.113.10")},
	})
	replaceTestLeafCertAndKeyData(t, oldCertData, options.ApiserverCertAndKeyName, globaloptions.CaCertAndKeyName, certutil.AltNames{
		DNSNames: []string{"old-api.example.test"},
		IPs:      []net.IP{net.ParseIP("203.0.113.11")},
	})
	replaceTestLeafCertAndKeyData(t, oldCertData, options.EtcdServerCertAndKeyName, options.EtcdCaCertAndKeyName, certutil.AltNames{
		DNSNames: []string{"karmada-etcd-2.karmada-etcd.karmada-system.svc.old.example"},
		IPs:      []net.IP{net.ParseIP("203.0.113.12")},
	})
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedKarmadaSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	updatedEtcdSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), etcdCertName, metav1.GetOptions{})
	assert.NoError(t, err)
	rotatedKarmadaCert := loadTestCertFromSecret(t, updatedKarmadaSecret, options.KarmadaCertAndKeyName)
	rotatedAPIServerCert := loadTestCertFromSecret(t, updatedKarmadaSecret, options.ApiserverCertAndKeyName)
	rotatedEtcdServerCert := loadTestCertFromSecret(t, updatedEtcdSecret, options.EtcdServerCertAndKeyName)
	assert.Contains(t, rotatedKarmadaCert.DNSNames, "old-webhook.example.test")
	assert.True(t, slices.ContainsFunc(rotatedKarmadaCert.IPAddresses, func(ip net.IP) bool { return ip.Equal(net.ParseIP("203.0.113.10")) }))
	assert.Contains(t, rotatedAPIServerCert.DNSNames, "old-api.example.test")
	assert.True(t, slices.ContainsFunc(rotatedAPIServerCert.IPAddresses, func(ip net.IP) bool { return ip.Equal(net.ParseIP("203.0.113.11")) }))
	assert.Contains(t, rotatedEtcdServerCert.DNSNames, "karmada-etcd-2.karmada-etcd.karmada-system.svc.old.example")
	assert.True(t, slices.ContainsFunc(rotatedEtcdServerCert.IPAddresses, func(ip net.IP) bool { return ip.Equal(net.ParseIP("203.0.113.12")) }))
}

func TestCommandInitOption_runCertRotateRefreshesExistingLocalAdminKubeconfig(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	dataPath := t.TempDir()
	kubeconfigPath := filepath.Join(dataPath, options.KarmadaKubeConfigName)
	serverURL := "https://karmada.example.test:32443"
	oldConfig := utils.CreateWithCerts(serverURL, options.UserName, options.ClusterName,
		[]byte(oldCertData[certFileName(globaloptions.CaCertAndKeyName)]),
		[]byte(oldCertData[keyFileName(options.KarmadaCertAndKeyName)]),
		[]byte(oldCertData[certFileName(options.KarmadaCertAndKeyName)]))
	assert.NoError(t, os.WriteFile(filepath.Join(dataPath, "old-ca.crt"), []byte(oldCertData[certFileName(globaloptions.CaCertAndKeyName)]), 0600))
	assert.NoError(t, os.WriteFile(filepath.Join(dataPath, "old-karmada.crt"), []byte(oldCertData[certFileName(options.KarmadaCertAndKeyName)]), 0600))
	oldConfig.Clusters[options.ClusterName].CertificateAuthorityData = nil
	oldConfig.Clusters[options.ClusterName].CertificateAuthority = "old-ca.crt"
	oldConfig.AuthInfos[options.UserName].ClientCertificateData = nil
	oldConfig.AuthInfos[options.UserName].ClientCertificate = "old-karmada.crt"
	oldConfig.AuthInfos[options.UserName].ClientKey = "/old/karmada.key"
	oldConfig.Clusters["other"] = &clientcmdapi.Cluster{Server: "https://other.example.test:6443"}
	oldConfig.AuthInfos["other-user"] = &clientcmdapi.AuthInfo{Token: "other-token"}
	oldConfig.Contexts["secondary"] = &clientcmdapi.Context{Cluster: "other", AuthInfo: "other-user"}
	oldConfig.CurrentContext = "secondary"
	configBytes, err := clientcmd.Write(*oldConfig)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(kubeconfigPath, configBytes, 0600))

	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KarmadaDataPath:          dataPath,
		KubeClientSet:            clientset,
	}

	err = opt.runCertRotate()
	assert.NoError(t, err)

	updatedSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	updatedConfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	assert.NoError(t, err)
	assert.Equal(t, "secondary", updatedConfig.CurrentContext)
	assert.Equal(t, serverURL, updatedConfig.Clusters[options.ClusterName].Server)
	assert.Equal(t, "https://other.example.test:6443", updatedConfig.Clusters["other"].Server)
	assert.Equal(t, "other-token", updatedConfig.AuthInfos["other-user"].Token)
	assert.Empty(t, updatedConfig.Clusters[options.ClusterName].CertificateAuthority)
	assert.Empty(t, updatedConfig.AuthInfos[options.UserName].ClientCertificate)
	assert.Empty(t, updatedConfig.AuthInfos[options.UserName].ClientKey)
	assert.True(t, bytes.Equal([]byte(updatedSecret.StringData[certFileName(globaloptions.CaCertAndKeyName)]), updatedConfig.Clusters[options.ClusterName].CertificateAuthorityData), "embedded CA certificate must be refreshed")
	assert.True(t, bytes.Equal([]byte(updatedSecret.StringData[certFileName(options.KarmadaCertAndKeyName)]), updatedConfig.AuthInfos[options.UserName].ClientCertificateData), "embedded client certificate must be refreshed")
	assert.True(t, bytes.Equal([]byte(updatedSecret.StringData[keyFileName(options.KarmadaCertAndKeyName)]), updatedConfig.AuthInfos[options.UserName].ClientKeyData), "embedded client key must be refreshed")
	fileInfo, err := os.Stat(kubeconfigPath)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm())
}

func TestCommandInitOption_runCertRotateRejectsInvalidLocalAdminKubeconfigBeforeSecretUpdates(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	dataPath := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dataPath, options.KarmadaKubeConfigName), []byte("invalid: ["), 0600))
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KarmadaDataPath:          dataPath,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load local Karmada kubeconfig")
	assertNoSecretUpdateActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateRejectsLocalAdminKubeconfigFromAnotherCluster(t *testing.T) {
	namespace := "karmada-system"
	targetCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, targetCertData)...)
	otherCA, _ := newTestCA(t, "other-karmada")
	dataPath := t.TempDir()
	kubeconfigPath := filepath.Join(dataPath, options.KarmadaKubeConfigName)
	otherConfig := utils.CreateWithCerts("https://other-karmada.example.test:32443", options.UserName, options.ClusterName,
		certpkg.EncodeCertPEM(otherCA),
		[]byte(targetCertData[keyFileName(options.KarmadaCertAndKeyName)]),
		[]byte(targetCertData[certFileName(options.KarmadaCertAndKeyName)]))
	configBytes, err := clientcmd.Write(*otherConfig)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(kubeconfigPath, configBytes, 0600))
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KarmadaDataPath:          dataPath,
		KubeClientSet:            clientset,
	}

	err = opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA does not match Secret")
	assertNoSecretUpdateActions(t, clientset.Actions())
	unchangedConfigBytes, readErr := os.ReadFile(kubeconfigPath)
	assert.NoError(t, readErr)
	assert.Equal(t, configBytes, unchangedConfigBytes)
}

func TestCommandInitOption_runCertRotateRejectsLocalAdminKubeconfigWithDifferentClientIdentity(t *testing.T) {
	namespace := "karmada-system"
	targetCertData := buildTestCertAndKeyData(t)
	otherCertData := maps.Clone(targetCertData)
	replaceTestLeafCertAndKeyData(t, otherCertData, options.KarmadaCertAndKeyName, globaloptions.CaCertAndKeyName, certutil.AltNames{})
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, targetCertData)...)
	dataPath := t.TempDir()
	kubeconfigPath := filepath.Join(dataPath, options.KarmadaKubeConfigName)
	otherConfig := utils.CreateWithCerts("https://other-karmada.example.test:32443", options.UserName, options.ClusterName,
		[]byte(otherCertData[certFileName(globaloptions.CaCertAndKeyName)]),
		[]byte(otherCertData[keyFileName(options.KarmadaCertAndKeyName)]),
		[]byte(otherCertData[certFileName(options.KarmadaCertAndKeyName)]))
	configBytes, err := clientcmd.Write(*otherConfig)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(kubeconfigPath, configBytes, 0600))
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KarmadaDataPath:          dataPath,
		KubeClientSet:            clientset,
	}

	err = opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client certificate does not match")
	assertNoSecretUpdateActions(t, clientset.Actions())
	unchangedConfigBytes, readErr := os.ReadFile(kubeconfigPath)
	assert.NoError(t, readErr)
	assert.Equal(t, configBytes, unchangedConfigBytes)
}

func TestCommandInitOption_runCertRotatePreservesExistingSecretMetadata(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	objects := buildExistingRotateSecrets(namespace, oldCertData)
	for _, object := range objects {
		secret := object.(*corev1.Secret)
		secret.Labels = map[string]string{
			"karmada.io/bootstrapping": "secret-defaults",
			"backup.example.io/name":   "daily",
		}
		secret.Annotations = map[string]string{"policy.example.io/managed": "true"}
	}
	clientset := fake.NewClientset(objects...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "daily", updatedSecret.Labels["backup.example.io/name"])
	assert.Equal(t, "true", updatedSecret.Annotations["policy.example.io/managed"])
}

func TestCommandInitOption_runCertRotateRejectsEtcdModeSwitch(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		ExternalEtcdServers:      "https://external-etcd.example.com:2379",
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "external etcd parameters cannot be used")
	assertNoSecretUpdateActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateRejectsExternalEtcdModeSwitch(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	objects := buildExistingExternalEtcdRotateSecrets(namespace, oldCertData)
	clientset := fake.NewClientset(objects...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal etcd parameters cannot be used")
	assertNoSecretUpdateActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateWithExistingExternalEtcdCerts(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingExternalEtcdRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		ExternalEtcdServers:      "https://external-etcd.example.com:2379",
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedEtcdSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), etcdCertName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Empty(t, updatedEtcdSecret.StringData[certFileName(options.EtcdServerCertAndKeyName)])
	assert.Empty(t, updatedEtcdSecret.StringData[keyFileName(options.EtcdServerCertAndKeyName)])
	assert.Equal(t, oldCertData[certFileName(options.EtcdCaCertAndKeyName)], updatedEtcdSecret.StringData[certFileName(options.EtcdCaCertAndKeyName)])

	updatedKarmadaSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, oldCertData[certFileName(options.EtcdClientCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(options.EtcdClientCertAndKeyName)])
	assert.Equal(t, oldCertData[keyFileName(options.EtcdClientCertAndKeyName)], updatedKarmadaSecret.StringData[keyFileName(options.EtcdClientCertAndKeyName)])
	assertNoInstallResourceActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateWithExternalEtcdCertFiles(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	certDir := t.TempDir()
	etcdCAFile := writeTestFile(t, certDir, certFileName(options.EtcdCaCertAndKeyName), oldCertData[certFileName(options.EtcdCaCertAndKeyName)])
	etcdClientCertFile := writeTestFile(t, certDir, certFileName(options.EtcdClientCertAndKeyName), oldCertData[certFileName(options.EtcdClientCertAndKeyName)])
	etcdClientKeyFile := writeTestFile(t, certDir, keyFileName(options.EtcdClientCertAndKeyName), oldCertData[keyFileName(options.EtcdClientCertAndKeyName)])
	clientset := fake.NewClientset(buildExistingExternalEtcdRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                   CertModeRotate,
		CertValidity:               365 * 24 * time.Hour,
		ExternalEtcdServers:        "https://external-etcd.example.com:2379",
		ExternalEtcdCACertPath:     etcdCAFile,
		ExternalEtcdClientCertPath: etcdClientCertFile,
		ExternalEtcdClientKeyPath:  etcdClientKeyFile,
		Namespace:                  namespace,
		HostClusterDomain:          "cluster.local",
		KarmadaAPIServerIP:         []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort:   32443,
		KubeClientSet:              clientset,
	}

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedEtcdSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), etcdCertName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, oldCertData[certFileName(options.EtcdCaCertAndKeyName)], updatedEtcdSecret.StringData[certFileName(options.EtcdCaCertAndKeyName)])
	assert.Empty(t, updatedEtcdSecret.StringData[keyFileName(options.EtcdCaCertAndKeyName)])

	updatedKarmadaSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, oldCertData[certFileName(options.EtcdClientCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(options.EtcdClientCertAndKeyName)])
	assert.Equal(t, oldCertData[keyFileName(options.EtcdClientCertAndKeyName)], updatedKarmadaSecret.StringData[keyFileName(options.EtcdClientCertAndKeyName)])
}

func TestCommandInitOption_runCertRotateRejectsExternalEtcdCredentialReplacement(t *testing.T) {
	oldCertData := buildTestCertAndKeyData(t)
	otherCertData := buildTestCertAndKeyData(t)
	tests := []struct {
		name           string
		caCertData     string
		clientCertData string
		clientKeyData  string
		wantError      string
	}{
		{
			name:           "different CA",
			caCertData:     otherCertData[certFileName(options.EtcdCaCertAndKeyName)],
			clientCertData: oldCertData[certFileName(options.EtcdClientCertAndKeyName)],
			clientKeyData:  oldCertData[keyFileName(options.EtcdClientCertAndKeyName)],
			wantError:      "external etcd CA certificate cannot be changed",
		},
		{
			name:           "different client credential",
			caCertData:     oldCertData[certFileName(options.EtcdCaCertAndKeyName)],
			clientCertData: otherCertData[certFileName(options.EtcdClientCertAndKeyName)],
			clientKeyData:  otherCertData[keyFileName(options.EtcdClientCertAndKeyName)],
			wantError:      "external etcd client certificate cannot be changed",
		},
		{
			name:           "mismatched client certificate and key",
			caCertData:     oldCertData[certFileName(options.EtcdCaCertAndKeyName)],
			clientCertData: oldCertData[certFileName(options.EtcdClientCertAndKeyName)],
			clientKeyData:  otherCertData[keyFileName(options.EtcdClientCertAndKeyName)],
			wantError:      "failed to load external etcd client certificate and key files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := "karmada-system"
			certDir := t.TempDir()
			clientset := fake.NewClientset(buildExistingExternalEtcdRotateSecrets(namespace, oldCertData)...)
			opt := &CommandInitOption{
				CertMode:                   CertModeRotate,
				CertValidity:               365 * 24 * time.Hour,
				ExternalEtcdServers:        "https://external-etcd.example.com:2379",
				ExternalEtcdCACertPath:     writeTestFile(t, certDir, certFileName(options.EtcdCaCertAndKeyName), tt.caCertData),
				ExternalEtcdClientCertPath: writeTestFile(t, certDir, certFileName(options.EtcdClientCertAndKeyName), tt.clientCertData),
				ExternalEtcdClientKeyPath:  writeTestFile(t, certDir, keyFileName(options.EtcdClientCertAndKeyName), tt.clientKeyData),
				Namespace:                  namespace,
				HostClusterDomain:          "cluster.local",
				KarmadaAPIServerIP:         []net.IP{utils.StringToNetIP("192.0.2.11")},
				KarmadaAPIServerNodePort:   32443,
				KubeClientSet:              clientset,
			}

			err := opt.runCertRotate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			assertNoSecretUpdateActions(t, clientset.Actions())
		})
	}
}

func TestCommandInitOption_runCertRotateWithCAFiles(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	certDir := t.TempDir()
	caCertFile := writeTestFile(t, certDir, certFileName(globaloptions.CaCertAndKeyName), oldCertData[certFileName(globaloptions.CaCertAndKeyName)])
	caKeyFile := writeTestFile(t, certDir, keyFileName(globaloptions.CaCertAndKeyName), oldCertData[keyFileName(globaloptions.CaCertAndKeyName)])
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		CaCertFile:               caCertFile,
		CaKeyFile:                caKeyFile,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.NoError(t, err)

	updatedKarmadaSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), globaloptions.KarmadaCertsName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, oldCertData[certFileName(globaloptions.CaCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(globaloptions.CaCertAndKeyName)])
	assert.Equal(t, oldCertData[keyFileName(globaloptions.CaCertAndKeyName)], updatedKarmadaSecret.StringData[keyFileName(globaloptions.CaCertAndKeyName)])
	assert.NotEqual(t, oldCertData[certFileName(options.KarmadaCertAndKeyName)], updatedKarmadaSecret.StringData[certFileName(options.KarmadaCertAndKeyName)])
}

func TestCommandInitOption_runCertRotateRejectsMismatchedCAFile(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	otherCertData := buildTestCertAndKeyData(t)
	certDir := t.TempDir()
	caCertFile := writeTestFile(t, certDir, certFileName(globaloptions.CaCertAndKeyName), otherCertData[certFileName(globaloptions.CaCertAndKeyName)])
	caKeyFile := writeTestFile(t, certDir, keyFileName(globaloptions.CaCertAndKeyName), otherCertData[keyFileName(globaloptions.CaCertAndKeyName)])
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		CaCertFile:               caCertFile,
		CaKeyFile:                caKeyFile,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ca-cert-file must match")
	assertNoSecretUpdateActions(t, clientset.Actions())
}

func TestCommandInitOption_updateCertsSecretsIncludesSecretIdentityOnUpdateFailure(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	clientset.PrependReactor("update", "secrets", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewInternalError(errors.New("forced update failure"))
	})
	opt := &CommandInitOption{
		Namespace:         namespace,
		HostClusterDomain: "cluster.local",
		KubeClientSet:     clientset,
		CertAndKeyFileData: stringMapToByteMap(map[string]string{
			certFileName(globaloptions.CaCertAndKeyName):         oldCertData[certFileName(globaloptions.CaCertAndKeyName)],
			keyFileName(globaloptions.CaCertAndKeyName):          oldCertData[keyFileName(globaloptions.CaCertAndKeyName)],
			certFileName(options.EtcdCaCertAndKeyName):           oldCertData[certFileName(options.EtcdCaCertAndKeyName)],
			keyFileName(options.EtcdCaCertAndKeyName):            oldCertData[keyFileName(options.EtcdCaCertAndKeyName)],
			certFileName(options.EtcdServerCertAndKeyName):       oldCertData[certFileName(options.EtcdServerCertAndKeyName)],
			keyFileName(options.EtcdServerCertAndKeyName):        oldCertData[keyFileName(options.EtcdServerCertAndKeyName)],
			certFileName(options.EtcdClientCertAndKeyName):       oldCertData[certFileName(options.EtcdClientCertAndKeyName)],
			keyFileName(options.EtcdClientCertAndKeyName):        oldCertData[keyFileName(options.EtcdClientCertAndKeyName)],
			certFileName(options.KarmadaCertAndKeyName):          oldCertData[certFileName(options.KarmadaCertAndKeyName)],
			keyFileName(options.KarmadaCertAndKeyName):           oldCertData[keyFileName(options.KarmadaCertAndKeyName)],
			certFileName(options.ApiserverCertAndKeyName):        oldCertData[certFileName(options.ApiserverCertAndKeyName)],
			keyFileName(options.ApiserverCertAndKeyName):         oldCertData[keyFileName(options.ApiserverCertAndKeyName)],
			certFileName(options.FrontProxyCaCertAndKeyName):     oldCertData[certFileName(options.FrontProxyCaCertAndKeyName)],
			keyFileName(options.FrontProxyCaCertAndKeyName):      oldCertData[keyFileName(options.FrontProxyCaCertAndKeyName)],
			certFileName(options.FrontProxyClientCertAndKeyName): oldCertData[certFileName(options.FrontProxyClientCertAndKeyName)],
			keyFileName(options.FrontProxyClientCertAndKeyName):  oldCertData[keyFileName(options.FrontProxyClientCertAndKeyName)],
		}),
	}

	err := opt.updateCertsSecrets()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to update Secret")
	assert.True(t, strings.Contains(err.Error(), namespace+"/"))
	assert.True(t, apierrors.IsInternalError(err))
}

func TestCommandInitOption_runCertRotateRequiresExistingSecrets(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	secrets := buildExistingRotateSecrets(namespace, oldCertData)
	clientset := fake.NewClientset(secrets[0])
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	for _, action := range clientset.Actions() {
		assert.NotEqual(t, "create", action.GetVerb())
		assert.NotEqual(t, "update", action.GetVerb())
	}
}

func TestCommandInitOption_runCertRotateRequiresExistingCAKey(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	delete(oldCertData, keyFileName(globaloptions.CaCertAndKeyName))
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err := opt.runCertRotate()
	assert.Error(t, err)
	assertNoInstallResourceActions(t, clientset.Actions())
	assertNoSecretUpdateActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateRejectsMismatchedKarmadaKeyBeforeSecretUpdates(t *testing.T) {
	namespace := "karmada-system"
	oldCertData := buildTestCertAndKeyData(t)
	otherKey, err := certpkg.NewPrivateKey(x509.ECDSA)
	assert.NoError(t, err)
	otherKeyPEM, err := certpkg.EncodePrivateKeyPEM(otherKey)
	assert.NoError(t, err)
	oldCertData[keyFileName(options.KarmadaCertAndKeyName)] = string(otherKeyPEM)
	clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
	opt := &CommandInitOption{
		CertMode:                 CertModeRotate,
		CertValidity:             365 * 24 * time.Hour,
		EtcdReplicas:             1,
		EtcdStorageMode:          etcdStorageModeEmptyDir,
		Namespace:                namespace,
		HostClusterDomain:        "cluster.local",
		KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
		KarmadaAPIServerNodePort: 32443,
		KubeClientSet:            clientset,
	}

	err = opt.runCertRotate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load existing certificate and key")
	assertNoSecretUpdateActions(t, clientset.Actions())
}

func TestCommandInitOption_runCertRotateRejectsInvalidCALifetimeBeforeSecretUpdates(t *testing.T) {
	now := time.Now().UTC()
	baseCertData := buildTestCertAndKeyData(t)
	tests := []struct {
		name        string
		caNotBefore time.Time
		caNotAfter  time.Time
	}{
		{
			name:        "expired CA",
			caNotBefore: now.Add(-48 * time.Hour),
			caNotAfter:  now.Add(-24 * time.Hour),
		},
		{
			name:        "CA expires before requested leaf certificate",
			caNotBefore: now.Add(-time.Hour),
			caNotAfter:  now.Add(30 * 24 * time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := "karmada-system"
			oldCertData := maps.Clone(baseCertData)
			replaceTestRootCAWithValidity(t, oldCertData, tt.caNotBefore, tt.caNotAfter)
			clientset := fake.NewClientset(buildExistingRotateSecrets(namespace, oldCertData)...)
			opt := &CommandInitOption{
				CertMode:                 CertModeRotate,
				CertValidity:             365 * 24 * time.Hour,
				EtcdReplicas:             1,
				EtcdStorageMode:          etcdStorageModeEmptyDir,
				Namespace:                namespace,
				HostClusterDomain:        "cluster.local",
				KarmadaAPIServerIP:       []net.IP{utils.StringToNetIP("192.0.2.11")},
				KarmadaAPIServerNodePort: 32443,
				KubeClientSet:            clientset,
			}

			err := opt.runCertRotate()
			assert.Error(t, err)
			assertNoSecretUpdateActions(t, clientset.Actions())
		})
	}
}

func TestValidateCAForLeaf(t *testing.T) {
	now := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	validNotAfter := now.Add(time.Hour)
	key, err := certpkg.NewPrivateKey(x509.ECDSA)
	assert.NoError(t, err)
	tests := []struct {
		name         string
		mutateCA     func(*x509.Certificate)
		leafNotAfter time.Time
		wantError    string
	}{
		{
			name:         "valid including equal CA and leaf expiry",
			leafNotAfter: validNotAfter,
		},
		{
			name: "valid without key usage extension",
			mutateCA: func(certificate *x509.Certificate) {
				certificate.KeyUsage = 0
			},
			leafNotAfter: validNotAfter,
		},
		{
			name: "invalid basic constraints",
			mutateCA: func(certificate *x509.Certificate) {
				certificate.BasicConstraintsValid = false
			},
			leafNotAfter: validNotAfter,
			wantError:    "not a valid certificate authority",
		},
		{
			name: "not a CA",
			mutateCA: func(certificate *x509.Certificate) {
				certificate.IsCA = false
			},
			leafNotAfter: validNotAfter,
			wantError:    "not a valid certificate authority",
		},
		{
			name: "key usage does not allow certificate signing",
			mutateCA: func(certificate *x509.Certificate) {
				certificate.KeyUsage = x509.KeyUsageDigitalSignature
			},
			leafNotAfter: validNotAfter,
			wantError:    "not authorized to sign certificates",
		},
		{
			name: "CA is not valid yet",
			mutateCA: func(certificate *x509.Certificate) {
				certificate.NotBefore = now.Add(time.Minute)
			},
			leafNotAfter: validNotAfter,
			wantError:    "is not valid before",
		},
		{
			name: "CA is expired",
			mutateCA: func(certificate *x509.Certificate) {
				certificate.NotAfter = now.Add(-time.Minute)
			},
			leafNotAfter: validNotAfter,
			wantError:    "expired at",
		},
		{
			name:         "requested certificate already expired",
			leafNotAfter: now,
			wantError:    "is not after the current time",
		},
		{
			name:         "requested certificate outlives CA",
			leafNotAfter: validNotAfter.Add(time.Minute),
			wantError:    "exceeds CA expiry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caCert := &x509.Certificate{
				Subject:               pkix.Name{CommonName: "test-ca"},
				NotBefore:             now.Add(-time.Hour),
				NotAfter:              validNotAfter,
				KeyUsage:              x509.KeyUsageCertSign,
				BasicConstraintsValid: true,
				IsCA:                  true,
			}
			if tt.mutateCA != nil {
				tt.mutateCA(caCert)
			}
			config := certpkg.NewCertConfig("test", nil, certutil.AltNames{}, &tt.leafNotAfter)
			err := validateCAForLeaf(&caMaterial{cert: caCert, key: key}, config, now)
			if tt.wantError == "" {
				assert.NoError(t, err)
				return
			}
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.wantError)
			}
		})
	}
}

func TestInitKarmadaAPIServer(t *testing.T) {
	// Create a fake clientset
	clientset := fake.NewClientset()

	// Create a new CommandInitOption
	initOption := &CommandInitOption{
		KubeClientSet: clientset,
		Namespace:     "test-namespace",
	}

	// Call the function
	err := initOption.initKarmadaAPIServer()

	// Check if the function returned an error
	if err != nil {
		t.Errorf("initKarmadaAPIServer() returned an error: %v", err)
	}

	// Check if the etcd StatefulSet was created
	_, err = clientset.AppsV1().StatefulSets(initOption.Namespace).Get(context.TODO(), etcdStatefulSetAndServiceName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("initKarmadaAPIServer() failed to create etcd StatefulSet: %v", err)
	}

	// Check if the karmada APIServer Deployment was created
	_, err = clientset.AppsV1().Deployments(initOption.Namespace).Get(context.TODO(), karmadaAPIServerDeploymentAndServiceName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("initKarmadaAPIServer() failed to create karmada APIServer Deployment: %v", err)
	}

	// Check if the karmada aggregated apiserver Deployment was created
	_, err = clientset.AppsV1().Deployments(initOption.Namespace).Get(context.TODO(), karmadaAggregatedAPIServerDeploymentAndServiceName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("initKarmadaAPIServer() failed to create karmada aggregated apiserver Deployment: %v", err)
	}
}

func TestCommandInitOption_initKarmadaComponent(t *testing.T) {
	// Create a fake kube clientset
	clientSet := fake.NewClientset()

	// Create a new CommandInitOption
	initOption := &CommandInitOption{
		KubeClientSet: clientSet,
		Namespace:     "test-namespace",
	}

	// Call the function
	err := initOption.initKarmadaComponent()

	// Assert that no error was returned
	if err != nil {
		t.Errorf("initKarmadaComponent returned an unexpected error: %v", err)
	}

	// Assert that the expected deployments were created
	expectedDeployments := []string{
		kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
		schedulerDeploymentNameAndServiceAccountName,
		controllerManagerDeploymentAndServiceName,
		webhookDeploymentAndServiceAccountAndServiceName,
	}
	for _, deploymentName := range expectedDeployments {
		_, err = clientSet.AppsV1().Deployments("test-namespace").Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Expected deployment %s was not created: %v", deploymentName, err)
		}
	}
}

func TestKubeRegistry(t *testing.T) {
	tests := []struct {
		name                 string
		opt                  *CommandInitOption
		expectedKubeRegistry string
	}{
		{
			name: "KubeImageRegistry is set",
			opt: &CommandInitOption{
				KubeImageRegistry: "my-registry",
			},
			expectedKubeRegistry: "my-registry",
		},
		{
			name: "KubeImageMirrorCountry is set to a supported value",
			opt: &CommandInitOption{
				KubeImageMirrorCountry: "CN",
			},
			expectedKubeRegistry: imageRepositories["cn"],
		},
		{
			name: "KubeImageMirrorCountry is set to an unsupported value",
			opt: &CommandInitOption{
				KubeImageMirrorCountry: "unsupported",
			},
			expectedKubeRegistry: imageRepositories["global"],
		},
		{
			name: "KubeImageRegistry and KubeImageMirrorCountry are not set, but ImageRegistry is set",
			opt: &CommandInitOption{
				ImageRegistry: "my-registry",
			},
			expectedKubeRegistry: "my-registry",
		},
		{
			name:                 "Neither KubeImageRegistry nor ImageRegistry are set, and KubeImageMirrorCountry is not set",
			opt:                  &CommandInitOption{},
			expectedKubeRegistry: imageRepositories["global"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.kubeRegistry()
			if result != tt.expectedKubeRegistry {
				t.Errorf("Unexpected result: %s", result)
			}
		})
	}
}

func TestKubeAPIServerImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "KarmadaAPIServerImage is set",
			opt: &CommandInitOption{
				KarmadaAPIServerImage: "my-karmada-image",
			},
			expected: "my-karmada-image",
		},
		{
			name: "KarmadaAPIServerImage is not set, should return the expected value based on kubeRegistry() and KubeImageTag",
			opt: &CommandInitOption{
				KubeImageTag: "1.20.1",
			},
			expected: imageRepositories["global"] + "/kube-apiserver:1.20.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.kubeAPIServerImage()
			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}

func TestKubeControllerManagerImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "KubeControllerManagerImage is set",
			opt: &CommandInitOption{
				KubeControllerManagerImage: "my-controller-manager-image",
			},
			expected: "my-controller-manager-image",
		},
		{
			name: "KubeControllerManagerImage is not set",
			opt: &CommandInitOption{
				KubeImageTag: "1.20.1",
			},
			expected: imageRepositories["global"] + "/kube-controller-manager:1.20.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opt.kubeControllerManagerImage()
			if got != tt.expected {
				t.Errorf("CommandInitOption.kubeControllerManagerImage() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEtcdImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "EtcdImage is set",
			opt: &CommandInitOption{
				EtcdImage: "my-etcd-image",
			},
			expected: "my-etcd-image",
		},
		{
			name:     "EtcdImage is not set",
			opt:      &CommandInitOption{},
			expected: imageRepositories["global"] + "/" + defaultEtcdImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.etcdImage()
			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}

func TestKarmadaSchedulerImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "ImageRegistry is set and KarmadaSchedulerImage is set to default value",
			opt: &CommandInitOption{
				ImageRegistry:         "my-registry",
				KarmadaSchedulerImage: DefaultKarmadaSchedulerImage,
			},
			expected: "my-registry/karmada-scheduler:" + karmadaRelease,
		},
		{
			name: "KarmadaSchedulerImage is set to a non-default value",
			opt: &CommandInitOption{
				KarmadaSchedulerImage: "my-scheduler-image",
			},
			expected: "my-scheduler-image",
		},
		{
			name: "ImageRegistry is not set and KarmadaSchedulerImage is set to default value",
			opt: &CommandInitOption{
				KarmadaSchedulerImage: DefaultKarmadaSchedulerImage,
			},
			expected: DefaultKarmadaSchedulerImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.karmadaSchedulerImage()

			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}

func TestCommandInitOption_parseEtcdNodeSelectorLabelsMap(t *testing.T) {
	tests := []struct {
		name     string
		opt      CommandInitOption
		wantErr  bool
		expected map[string]string
	}{
		{
			name: "Valid labels",
			opt: CommandInitOption{
				EtcdNodeSelectorLabels: "kubernetes.io/os=linux,hello=world",
			},
			wantErr: false,
			expected: map[string]string{
				"kubernetes.io/os": "linux",
				"hello":            "world",
			},
		},
		{
			name: "Invalid labels without equal sign",
			opt: CommandInitOption{
				EtcdNodeSelectorLabels: "invalidlabel",
			},
			wantErr:  true,
			expected: nil,
		},
		{
			name: "Labels with extra spaces",
			opt: CommandInitOption{
				EtcdNodeSelectorLabels: "  key1 = value1 , key2=value2  ",
			},
			wantErr: false,
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opt.parseEtcdNodeSelectorLabelsMap()
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEtcdNodeSelectorLabelsMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(tt.opt.EtcdNodeSelectorLabelsMap, tt.expected) {
				t.Errorf("parseEtcdNodeSelectorLabelsMap() = %v, want %v", tt.opt.EtcdNodeSelectorLabelsMap, tt.expected)
			}
		})
	}
}

func TestParseInitConfig(t *testing.T) {
	cfg := &config.KarmadaInitConfig{
		Spec: config.KarmadaInitSpec{
			WaitComponentReadyTimeout: 200,
			KarmadaDataPath:           "/etc/karmada",
			KarmadaPKIPath:            "/etc/karmada/pki",
			KarmadaCRDs:               "https://github.com/karmada-io/karmada/releases/download/test/crds.tar.gz",
			Certificates: config.Certificates{
				CACertFile:     "/path/to/ca.crt",
				CAKeyFile:      "/path/to/ca.key",
				ExternalDNS:    []string{"dns1", "dns2"},
				ExternalIP:     []string{"1.2.3.4", "5.6.7.8"},
				ValidityPeriod: metav1.Duration{Duration: parseDuration("8760h")},
			},
			Etcd: config.Etcd{
				Local: &config.LocalEtcd{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "etcd-image",
							Tag:        "latest",
						},
						Replicas: 3,
					},
					DataPath: "/data/dir",
					PVCSize:  "5Gi",
					NodeSelectorLabels: map[string]string{
						"key": "value",
					},
					StorageClassesName: "fast",
					StorageMode:        "PVC",
				},
				External: &config.ExternalEtcd{
					CAFile:    "/etc/ssl/certs/ca-certificates.crt",
					CertFile:  "/path/to/certificate.pem",
					KeyFile:   "/path/to/privatekey.pem",
					Endpoints: []string{"https://example.com:8443"},
					KeyPrefix: "ext-",
				},
			},
			HostCluster: config.HostCluster{
				Kubeconfig: "/path/to/kubeconfig",
				Context:    "test-context",
				Domain:     "cluster.local",
			},
			Images: config.Images{
				KubeImageTag:           "v1.21.0",
				KubeImageRegistry:      "registry",
				KubeImageMirrorCountry: "cn",
				ImagePullPolicy:        corev1.PullIfNotPresent,
				ImagePullSecrets:       []string{"secret1", "secret2"},
				PrivateRegistry: &config.ImageRegistry{
					Registry: "test-registry",
				},
			},
			Components: config.KarmadaComponents{
				KarmadaAPIServer: &config.KarmadaAPIServer{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "apiserver-image",
							Tag:        "latest",
						},
						Replicas: 2,
					},
					AdvertiseAddress: "192.168.1.1",
					Networking: config.Networking{
						Namespace: "test-namespace",
						Port:      32443,
					},
				},
				KarmadaControllerManager: &config.KarmadaControllerManager{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "controller-manager-image",
							Tag:        "latest",
						},
						Replicas: 2,
					},
				},
				KarmadaScheduler: &config.KarmadaScheduler{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "scheduler-image",
							Tag:        "latest",
						},
						Replicas: 2,
					},
				},
				KarmadaWebhook: &config.KarmadaWebhook{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "webhook-image",
							Tag:        "latest",
						},
						Replicas: 2,
					},
				},
				KarmadaAggregatedAPIServer: &config.KarmadaAggregatedAPIServer{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "aggregated-apiserver-image",
							Tag:        "latest",
						},
						Replicas: 2,
					},
				},
				KubeControllerManager: &config.KubeControllerManager{
					CommonSettings: config.CommonSettings{
						Image: config.Image{
							Repository: "kube-controller-manager-image",
							Tag:        "latest",
						},
						Replicas: 2,
					},
				},
			},
		},
	}

	opt := &CommandInitOption{CertMode: CertModeRotate}
	err := opt.parseInitConfig(cfg)
	assert.NoError(t, err)
	assert.Equal(t, CertModeRotate, opt.CertMode)
	assert.Equal(t, "test-namespace", opt.Namespace)
	assert.Equal(t, "/path/to/kubeconfig", opt.KubeConfig)
	assert.Equal(t, "test-registry", opt.ImageRegistry)
	assert.Equal(t, 200, opt.WaitComponentReadyTimeout)
	assert.Equal(t, "dns1,dns2", opt.ExternalDNS)
	assert.Equal(t, "1.2.3.4,5.6.7.8", opt.ExternalIP)
	assert.Equal(t, parseDuration("8760h"), opt.CertValidity)
	assert.Equal(t, "etcd-image:latest", opt.EtcdImage)
	assert.Equal(t, "/data/dir", opt.EtcdHostDataPath)
	assert.Equal(t, "5Gi", opt.EtcdPersistentVolumeSize)
	assert.Equal(t, "key=value", opt.EtcdNodeSelectorLabels)
	assert.Equal(t, "fast", opt.StorageClassesName)
	assert.Equal(t, "PVC", opt.EtcdStorageMode)
	assert.Equal(t, int32(3), opt.EtcdReplicas)
	assert.Equal(t, "apiserver-image:latest", opt.KarmadaAPIServerImage)
	assert.Equal(t, "192.168.1.1", opt.KarmadaAPIServerAdvertiseAddress)
	assert.Equal(t, int32(2), opt.KarmadaAPIServerReplicas)
	assert.Equal(t, "registry", opt.KubeImageRegistry)
	assert.Equal(t, "cn", opt.KubeImageMirrorCountry)
	assert.Equal(t, "IfNotPresent", opt.ImagePullPolicy)
	assert.Equal(t, []string{"secret1", "secret2"}, opt.PullSecrets)
	assert.Equal(t, "https://github.com/karmada-io/karmada/releases/download/test/crds.tar.gz", opt.CRDs)
}

func TestParseInitConfig_MissingFields(t *testing.T) {
	cfg := &config.KarmadaInitConfig{
		Spec: config.KarmadaInitSpec{
			Components: config.KarmadaComponents{
				KarmadaAPIServer: &config.KarmadaAPIServer{
					Networking: config.Networking{
						Namespace: "test-namespace",
					},
				},
			},
		},
	}

	opt := &CommandInitOption{}
	err := opt.parseInitConfig(cfg)
	assert.NoError(t, err)
	assert.Equal(t, "test-namespace", opt.Namespace)
	assert.Empty(t, opt.KubeConfig)
	assert.Empty(t, opt.KubeImageTag)
}

func buildTestCertAndKeyData(t *testing.T) map[string]string {
	t.Helper()
	return buildTestCertAndKeyDataWithLeafNotAfter(t, time.Now().Add(365*24*time.Hour).UTC())
}

func buildTestCertAndKeyDataWithLeafNotAfter(t *testing.T, leafNotAfter time.Time) map[string]string {
	t.Helper()

	rootCA, rootKey := newTestCA(t, "karmada")
	frontProxyCA, frontProxyKey := newTestCA(t, "front-proxy-ca")
	etcdCA, etcdKey := newTestCA(t, "etcd-ca")

	certData := map[string]string{}
	setTestCertAndKeyData(t, certData, globaloptions.CaCertAndKeyName, rootCA, rootKey)
	setTestCertAndKeyData(t, certData, options.FrontProxyCaCertAndKeyName, frontProxyCA, frontProxyKey)
	setTestCertAndKeyData(t, certData, options.EtcdCaCertAndKeyName, etcdCA, etcdKey)
	setTestLeafCertAndKeyData(t, certData, options.KarmadaCertAndKeyName, rootCA, rootKey, certpkg.NewCertConfig("system:admin", []string{"system:masters"}, certutil.AltNames{}, &leafNotAfter))
	setTestLeafCertAndKeyData(t, certData, options.ApiserverCertAndKeyName, rootCA, rootKey, certpkg.NewCertConfig("karmada-apiserver", []string{}, certutil.AltNames{}, &leafNotAfter))
	setTestLeafCertAndKeyData(t, certData, options.FrontProxyClientCertAndKeyName, frontProxyCA, frontProxyKey, certpkg.NewCertConfig("front-proxy-client", []string{}, certutil.AltNames{}, &leafNotAfter))
	setTestLeafCertAndKeyData(t, certData, options.EtcdServerCertAndKeyName, etcdCA, etcdKey, certpkg.NewCertConfig("karmada-etcd-server", []string{}, certutil.AltNames{}, &leafNotAfter))
	setTestLeafCertAndKeyData(t, certData, options.EtcdClientCertAndKeyName, etcdCA, etcdKey, certpkg.NewCertConfig("karmada-etcd-client", []string{}, certutil.AltNames{}, &leafNotAfter))
	return certData
}

func newTestCA(t *testing.T, commonName string) (*x509.Certificate, crypto.Signer) {
	t.Helper()
	caCert, caKey, err := certpkg.NewCACertAndKey(commonName)
	assert.NoError(t, err)
	return caCert, *caKey
}

func replaceTestRootCAWithValidity(t *testing.T, certData map[string]string, notBefore, notAfter time.Time) {
	t.Helper()
	rootKey, err := certpkg.NewPrivateKey(x509.ECDSA)
	assert.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "karmada"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, rootKey.Public(), rootKey)
	assert.NoError(t, err)
	rootCA, err := x509.ParseCertificate(certDER)
	assert.NoError(t, err)
	setTestCertAndKeyData(t, certData, globaloptions.CaCertAndKeyName, rootCA, rootKey)

	leafNotAfter := time.Now().Add(time.Hour).UTC()
	karmadaConfig := certpkg.NewCertConfig("system:admin", []string{"system:masters"}, certutil.AltNames{}, &leafNotAfter)
	karmadaConfig.PublicKeyAlgorithm = x509.ECDSA
	setTestLeafCertAndKeyData(t, certData, options.KarmadaCertAndKeyName, rootCA, rootKey, karmadaConfig)
	apiserverConfig := certpkg.NewCertConfig("karmada-apiserver", []string{}, certutil.AltNames{}, &leafNotAfter)
	apiserverConfig.PublicKeyAlgorithm = x509.ECDSA
	setTestLeafCertAndKeyData(t, certData, options.ApiserverCertAndKeyName, rootCA, rootKey, apiserverConfig)
}

func setTestCertAndKeyData(t *testing.T, certData map[string]string, name string, certificate *x509.Certificate, key crypto.Signer) {
	t.Helper()
	keyPEM, err := certpkg.EncodePrivateKeyPEM(key)
	assert.NoError(t, err)
	certData[certFileName(name)] = string(certpkg.EncodeCertPEM(certificate))
	certData[keyFileName(name)] = string(keyPEM)
}

func setTestLeafCertAndKeyData(t *testing.T, certData map[string]string, name string, caCert *x509.Certificate, caKey crypto.Signer, certConfig *certpkg.CertsConfig) {
	t.Helper()
	leafCert, leafKey, err := certpkg.NewCertAndKey(caCert, caKey, certConfig)
	assert.NoError(t, err)
	setTestCertAndKeyData(t, certData, name, leafCert, leafKey)
}

func replaceTestLeafCertAndKeyData(t *testing.T, certData map[string]string, name, caName string, altNames certutil.AltNames) {
	t.Helper()
	existingCert, err := certpkg.LoadCertificatePEM([]byte(certData[certFileName(name)]))
	assert.NoError(t, err)
	caCert, caKey, err := certpkg.LoadCertAndKeyPEM([]byte(certData[certFileName(caName)]), []byte(certData[keyFileName(caName)]))
	assert.NoError(t, err)
	notAfter := time.Now().Add(365 * 24 * time.Hour).UTC()
	certConfig := certpkg.NewCertConfig(existingCert.Subject.CommonName, existingCert.Subject.Organization, altNames, &notAfter)
	certConfig.Usages = slices.Clone(existingCert.ExtKeyUsage)
	setTestLeafCertAndKeyData(t, certData, name, caCert, caKey, certConfig)
}

func buildExistingRotateSecrets(namespace string, certData map[string]string) []runtime.Object {
	karmadaCertData := map[string]string{}
	maps.Copy(karmadaCertData, certData)
	etcdCertData := map[string]string{
		certFileName(options.EtcdCaCertAndKeyName):     certData[certFileName(options.EtcdCaCertAndKeyName)],
		keyFileName(options.EtcdCaCertAndKeyName):      certData[keyFileName(options.EtcdCaCertAndKeyName)],
		certFileName(options.EtcdServerCertAndKeyName): certData[certFileName(options.EtcdServerCertAndKeyName)],
		keyFileName(options.EtcdServerCertAndKeyName):  certData[keyFileName(options.EtcdServerCertAndKeyName)],
	}

	objects := []runtime.Object{
		testSecret(namespace, globaloptions.KarmadaCertsName, karmadaCertData),
		testSecret(namespace, etcdCertName, etcdCertData),
		testSecret(namespace, webhookCertsName, map[string]string{
			"tls.crt": certData[certFileName(options.KarmadaCertAndKeyName)],
			"tls.key": certData[keyFileName(options.KarmadaCertAndKeyName)],
		}),
	}

	for _, name := range karmadaConfigList {
		objects = append(objects, testSecret(namespace, name, map[string]string{util.KarmadaConfigFieldName: "old-config"}))
	}
	return objects
}

func buildExistingExternalEtcdRotateSecrets(namespace string, certData map[string]string) []runtime.Object {
	objects := buildExistingRotateSecrets(namespace, certData)
	for _, object := range objects {
		secret := object.(*corev1.Secret)
		if secret.Name != etcdCertName {
			continue
		}
		delete(secret.StringData, certFileName(options.EtcdServerCertAndKeyName))
		delete(secret.StringData, keyFileName(options.EtcdServerCertAndKeyName))
		delete(secret.Data, certFileName(options.EtcdServerCertAndKeyName))
		delete(secret.Data, keyFileName(options.EtcdServerCertAndKeyName))
	}
	return objects
}

func writeTestFile(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	assert.NoError(t, os.WriteFile(path, []byte(data), 0600))
	return path
}

func loadTestCertFromData(t *testing.T, certData map[string]string, name string) *x509.Certificate {
	t.Helper()
	certificate, err := certpkg.LoadCertificatePEM([]byte(certData[certFileName(name)]))
	assert.NoError(t, err)
	return certificate
}

func loadTestCertFromSecret(t *testing.T, secret *corev1.Secret, name string) *x509.Certificate {
	t.Helper()
	certificate, err := certpkg.LoadCertificatePEM([]byte(secret.StringData[certFileName(name)]))
	assert.NoError(t, err)
	return certificate
}

func testSecret(namespace, name string, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
		Data:       stringMapToByteMap(data),
	}
}

func stringMapToByteMap(data map[string]string) map[string][]byte {
	result := map[string][]byte{}
	for key, value := range data {
		result[key] = []byte(value)
	}
	return result
}

func assertNoInstallResourceActions(t *testing.T, actions []k8stesting.Action) {
	t.Helper()
	for _, action := range actions {
		switch action.GetResource().Resource {
		case "deployments", "statefulsets", "services":
			t.Fatalf("unexpected %s action for %s during certificate rotation", action.GetVerb(), action.GetResource().Resource)
		}
	}
}

func assertNoSecretUpdateActions(t *testing.T, actions []k8stesting.Action) {
	t.Helper()
	for _, action := range actions {
		if action.GetResource().Resource == "secrets" && action.GetVerb() == "update" {
			t.Fatalf("unexpected secret update action during failed certificate rotation")
		}
	}
}

// parseDuration parses a duration string and returns the corresponding time.Duration value.
// If the parsing fails, it returns a duration of 0.
func parseDuration(durationStr string) time.Duration {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0
	}
	return duration
}
