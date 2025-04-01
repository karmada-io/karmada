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
	"context"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/config"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.opt.Validate("parentCommand"); (err != nil) != tt.wantErr {
				t.Errorf("%s err = %v, want %v", tt.name, err.Error(), tt.errorMsg)
			}
		})
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

func TestInitKarmadaAPIServer(t *testing.T) {
	// Create a fake clientset
	clientset := fake.NewSimpleClientset()

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
	clientSet := fake.NewSimpleClientset()

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

func TestEtcdInitImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "ImageRegistry is set and EtcdInitImage is set to default value",
			opt: &CommandInitOption{
				ImageRegistry: "my-registry",
				EtcdInitImage: DefaultInitImage,
			},
			expected: "my-registry/alpine:3.21.3",
		},
		{
			name: "EtcdInitImage is set to a non-default value",
			opt: &CommandInitOption{
				EtcdInitImage: "my-etcd-init-image",
			},
			expected: "my-etcd-init-image",
		},
		{
			name: "ImageRegistry is not set and EtcdInitImage is set to default value",
			opt: &CommandInitOption{
				EtcdInitImage: DefaultInitImage,
			},
			expected: DefaultInitImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.etcdInitImage()
			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
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
					InitImage: config.Image{
						Repository: "init-image",
						Tag:        "latest",
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

	opt := &CommandInitOption{}
	err := opt.parseInitConfig(cfg)
	assert.NoError(t, err)
	assert.Equal(t, "test-namespace", opt.Namespace)
	assert.Equal(t, "/path/to/kubeconfig", opt.KubeConfig)
	assert.Equal(t, "test-registry", opt.ImageRegistry)
	assert.Equal(t, 200, opt.WaitComponentReadyTimeout)
	assert.Equal(t, "dns1,dns2", opt.ExternalDNS)
	assert.Equal(t, "1.2.3.4,5.6.7.8", opt.ExternalIP)
	assert.Equal(t, parseDuration("8760h"), opt.CertValidity)
	assert.Equal(t, "etcd-image:latest", opt.EtcdImage)
	assert.Equal(t, "init-image:latest", opt.EtcdInitImage)
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

// parseDuration parses a duration string and returns the corresponding time.Duration value.
// If the parsing fails, it returns a duration of 0.
func parseDuration(durationStr string) time.Duration {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0
	}
	return duration
}
