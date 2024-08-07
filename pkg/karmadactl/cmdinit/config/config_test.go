/*
Copyright 2024 The Karmada Authors.

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

package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testConfig = `
apiVersion: config.karmada.io/v1alpha1
kind: KarmadaInitConfig
metadata:
 name: karmada-init
spec:
 certificates:
   caCertFile: "/etc/karmada/pki/ca.crt"
   caKeyFile: "/etc/karmada/pki/ca.key"
   externalDNS:
     - "localhost"
     - "example.com"
   externalIP:
     - "192.168.1.2"
     - "172.16.1.2"
   validityPeriod: "8760h0m0s"
 etcd:
   local:
     Repository: "registry.k8s.io/etcd"
     Tag: "latest"
     dataPath: "/var/lib/karmada-etcd"
     initImage:
       repository: "alpine"
       tag: "3.19.1"
     nodeSelectorLabels:
       karmada.io/etcd: "true"
     pvcSize: "5Gi"
     replicas: 3
     storageMode: "PVC"
   external:
     endpoints:
       - "https://example.com:8443"
     caFile: "/path/to/your/ca.crt"
     certFile: "/path/to/your/cert.crt"
     keyFile: "/path/to/your/key.key"
     keyPrefix: "ext-"
 hostCluster:
   apiEndpoint: "https://kubernetes.example.com"
   kubeconfig: "/root/.kube/config"
   context: "karmada-host"
   domain: "cluster.local"
 images:
   imagePullPolicy: "IfNotPresent"
   imagePullSecrets:
     - "PullSecret1"
     - "PullSecret2"
   kubeImageMirrorCountry: "cn"
   kubeImageRegistry: "registry.cn-hangzhou.aliyuncs.com/google_containers"
   kubeImageTag: "v1.29.6"
   privateRegistry:
     registry: "my.private.registry"
 components:
   karmadaAPIServer:
     repository: "karmada/kube-apiserver"
     tag: "v1.29.6"
     replicas: 1
     advertiseAddress: "192.168.1.100"
     serviceType: "NodePort"
     networking:
       namespace: "karmada-system"
       port: 32443
   karmadaAggregatedAPIServer:
     repository: "karmada/karmada-aggregated-apiserver"
     tag: "v0.0.0-master"
     replicas: 1
   kubeControllerManager:
     repository: "karmada/kube-controller-manager"
     tag: "v1.29.6"
     replicas: 1
   karmadaControllerManager:
     repository: "karmada/karmada-controller-manager"
     tag: "v0.0.0-master"
     replicas: 1
   karmadaScheduler:
     repository: "karmada/karmada-scheduler"
     tag: "v0.0.0-master"
     replicas: 1
   karmadaWebhook:
     repository: "karmada/karmada-webhook"
     tag: "v0.0.0-master"
     replicas: 1
 karmadaDataPath: "/etc/karmada"
 karmadaPkiPath: "/etc/karmada/pki"
 karmadaCrds: "https://github.com/karmada-io/karmada/releases/download/test/crds.tar.gz"
 waitComponentReadyTimeout: 120
`

const invalidTestConfig = `
apiVersion: v1alpha1
kind: KarmadaInitConfig
metadata:
  name: karmada-init
spec:
  waitComponentReadyTimeout: "invalid-int"
`

func TestLoadInitConfiguration(t *testing.T) {
	expectedConfig := &KarmadaInitConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KarmadaInitConfig",
			APIVersion: "config.karmada.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "karmada-init",
		},
		Spec: KarmadaInitSpec{
			WaitComponentReadyTimeout: 120,
			KarmadaDataPath:           "/etc/karmada",
			KarmadaPkiPath:            "/etc/karmada/pki",
			KarmadaCrds:               "https://github.com/karmada-io/karmada/releases/download/test/crds.tar.gz",
			Certificates: Certificates{
				CACertFile: "/etc/karmada/pki/ca.crt",
				CAKeyFile:  "/etc/karmada/pki/ca.key",
				ExternalDNS: []string{
					"localhost",
					"example.com",
				},
				ExternalIP: []string{
					"192.168.1.2",
					"172.16.1.2",
				},
				ValidityPeriod: metav1.Duration{Duration: parseDuration("8760h")},
			},
			Etcd: Etcd{
				Local: &LocalEtcd{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "registry.k8s.io/etcd",
							Tag:        "latest",
						},
						Replicas: 3,
					},
					InitImage: Image{
						Repository: "alpine",
						Tag:        "3.19.1",
					},
					DataPath: "/var/lib/karmada-etcd",
					PVCSize:  "5Gi",
					NodeSelectorLabels: map[string]string{
						"karmada.io/etcd": "true",
					},
					StorageMode: "PVC",
				},
				External: &ExternalEtcd{
					Endpoints: []string{
						"https://example.com:8443",
					},
					CAFile:    "/path/to/your/ca.crt",
					CertFile:  "/path/to/your/cert.crt",
					KeyFile:   "/path/to/your/key.key",
					KeyPrefix: "ext-",
				},
			},
			HostCluster: HostCluster{
				APIEndpoint: "https://kubernetes.example.com",
				Kubeconfig:  "/root/.kube/config",
				Context:     "karmada-host",
				Domain:      "cluster.local",
			},
			Images: Images{
				ImagePullPolicy:        corev1.PullIfNotPresent,
				ImagePullSecrets:       []string{"PullSecret1", "PullSecret2"},
				KubeImageMirrorCountry: "cn",
				KubeImageRegistry:      "registry.cn-hangzhou.aliyuncs.com/google_containers",
				KubeImageTag:           "v1.29.6",
				PrivateRegistry: &ImageRegistry{
					Registry: "my.private.registry",
				},
			},
			Components: KarmadaComponents{
				KarmadaAPIServer: &KarmadaAPIServer{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "karmada/kube-apiserver",
							Tag:        "v1.29.6",
						},
						Replicas: 1,
					},
					AdvertiseAddress: "192.168.1.100",
					Networking: Networking{
						Namespace: "karmada-system",
						Port:      32443,
					},
				},
				KarmadaAggregatedAPIServer: &KarmadaAggregatedAPIServer{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "karmada/karmada-aggregated-apiserver",
							Tag:        "v0.0.0-master",
						},
						Replicas: 1,
					},
				},
				KubeControllerManager: &KubeControllerManager{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "karmada/kube-controller-manager",
							Tag:        "v1.29.6",
						},
						Replicas: 1,
					},
				},
				KarmadaControllerManager: &KarmadaControllerManager{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "karmada/karmada-controller-manager",
							Tag:        "v0.0.0-master",
						},
						Replicas: 1,
					},
				},
				KarmadaScheduler: &KarmadaScheduler{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "karmada/karmada-scheduler",
							Tag:        "v0.0.0-master",
						},
						Replicas: 1,
					},
				},
				KarmadaWebhook: &KarmadaWebhook{
					CommonSettings: CommonSettings{
						Image: Image{
							Repository: "karmada/karmada-webhook",
							Tag:        "v0.0.0-master",
						},
						Replicas: 1,
					},
				},
			},
		},
	}

	t.Run("Test Load Valid Configuration", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(testConfig))
		assert.NoError(t, err)
		err = tmpFile.Close()
		assert.NoError(t, err)

		config, err := LoadInitConfiguration(tmpFile.Name())
		assert.NoError(t, err)
		assert.Equal(t, expectedConfig, config)
	})

	t.Run("Test Load Invalid Configuration", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "invalid-config-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(invalidTestConfig))
		assert.NoError(t, err)
		err = tmpFile.Close()
		assert.NoError(t, err)

		_, err = LoadInitConfiguration(tmpFile.Name())
		assert.Error(t, err)
	})

	t.Run("Test Load Non-Existent Configuration", func(t *testing.T) {
		_, err := LoadInitConfiguration("non-existent-file.yaml")
		assert.Error(t, err)
	})
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
