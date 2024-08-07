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

	"github.com/stretchr/testify/assert"
)

const testConfig = `
apiVersion: config.karmada.io/v1beta1
kind: InitConfiguration
generalConfig:
  namespace: "karmada-system"
  kubeConfigPath: "/etc/karmada/kubeconfig"
  kubeImageTag: "v1.21.0"
  context: "karmada-context"
  storageClassesName: "fast"
  privateImageRegistry: "local.registry.com"
  waitComponentReadyTimeout: 120
  port: 32443

certificateConfig:
  externalDNS:
    - "www.karmada.io"
  externalIP:
    - "10.235.1.2"
  validityPeriod: "8760h"
  caCertFile: "/etc/karmada/pki/ca.crt"
  caCertKeyFile: "/etc/karmada/pki/ca.key"
  extraArgs:
    - name: "arg1"
      value: "value1"

etcdConfig:
  local:
    image: "local.registry.com/library/etcd:3.5.13-0"
    initImage: "docker.io/alpine:3.19.1"
    dataDir: "/var/lib/karmada-etcd"
    pvcSize: "5Gi"
    nodeSelectorLabels:
      karmada.io/etcd: true
    storageMode: "PVC"
    replicas: 3
    extraArgs:
      - name: "etcd-arg1"
        value: "etcd-value1"
  external:
    externalCAPath: "/etc/ssl/certs/ca-certificates.crt"
    externalCertPath: "/path/to/your/certificate.pem"
    externalKeyPath: "/path/to/your/privatekey.pem"
    externalServers: "https://example.com:8443"
    externalPrefix: "ext-"
    extraArgs:
      - name: "ext-etcd-arg1"
        value: "ext-etcd-value1"

karmadaControlPlaneConfig:
  apiServer:
    image: "karmada-apiserver:latest"
    advertiseAddress: "192.168.1.2"
    replicas: 3
    nodePort: 32443
    extraArgs:
      - name: "api-arg1"
        value: "api-value1"
  controllerManager:
    image: "karmada-controller-manager:latest"
    replicas: 3
    extraArgs:
      - name: "cm-arg1"
        value: "cm-value1"
  scheduler:
    image: "karmada-scheduler:latest"
    replicas: 3
    extraArgs:
      - name: "sched-arg1"
        value: "sched-value1"
  webhook:
    image: "karmada-webhook:latest"
    replicas: 3
    extraArgs:
      - name: "webhook-arg1"
        value: "webhook-value1"
  aggregatedAPIServerConfig:
    image: "karmada-aggregated-apiserver:latest"
    replicas: 3
    extraArgs:
      - name: "aggregated-arg1"
        value: "aggregated-value1"
  kubeControllerManagerConfig:
    image: "kube-controller-manager:latest"
    replicas: 3
    extraArgs:
      - name: "kcm-arg1"
        value: "kcm-value1"
  dataPath: "/etc/karmada"
  pkiPath: "/etc/karmada/pki"
  crds: "/etc/karmada/crds"
  hostClusterDomain: "cluster.local"

imageConfig:
  kubeImageTag: "v1.21.0"
  kubeImageRegistry: "registry.cn-hangzhou.aliyuncs.com/google_containers"
  kubeImageMirrorCountry: "cn"
  imagePullPolicy: "IfNotPresent"
  imagePullSecrets:
    - "PullSecret1"
    - "PullSecret2"
`

const invalidTestConfig = `
apiVersion: config.karmada.io/v1beta1
kind: InitConfiguration
generalConfig:
  namespace: karmada-system
  kubeConfigPath: /etc/karmada/kubeconfig
  kubeImageTag: v1.21.0
  context: karmada-context
  storageClassesName: fast
  privateImageRegistry: local.registry.com
  waitComponentReadyTimeout: invalid-int
  port: 32443
`

func TestLoadInitConfiguration(t *testing.T) {
	expectedConfig := &InitConfiguration{
		GeneralConfig: GeneralConfig{
			Namespace:                 "karmada-system",
			KubeConfigPath:            "/etc/karmada/kubeconfig",
			KubeImageTag:              "v1.21.0",
			Context:                   "karmada-context",
			StorageClassesName:        "fast",
			PrivateImageRegistry:      "local.registry.com",
			WaitComponentReadyTimeout: 120,
			Port:                      32443,
		},
		CertificateConfig: CertificateConfig{
			ExternalDNS:    []string{"www.karmada.io"},
			ExternalIP:     []string{"10.235.1.2"},
			ValidityPeriod: "8760h",
			CaCertFile:     "/etc/karmada/pki/ca.crt",
			CaCertKeyFile:  "/etc/karmada/pki/ca.key",
			ExtraArgs:      []Arg{{Name: "arg1", Value: "value1"}},
		},
		EtcdConfig: EtcdConfig{
			Local: &LocalEtcd{
				Image:     "local.registry.com/library/etcd:3.5.13-0",
				InitImage: "docker.io/alpine:3.19.1",
				DataDir:   "/var/lib/karmada-etcd",
				PVCSize:   "5Gi",
				NodeSelectorLabels: map[string]string{
					"karmada.io/etcd": "true",
				},
				StorageMode: "PVC",
				Replicas:    3,
				ExtraArgs:   []Arg{{Name: "etcd-arg1", Value: "etcd-value1"}},
			},
			External: &ExternalEtcd{
				ExternalCAPath:   "/etc/ssl/certs/ca-certificates.crt",
				ExternalCertPath: "/path/to/your/certificate.pem",
				ExternalKeyPath:  "/path/to/your/privatekey.pem",
				ExternalServers:  "https://example.com:8443",
				ExternalPrefix:   "ext-",
				ExtraArgs:        []Arg{{Name: "ext-etcd-arg1", Value: "ext-etcd-value1"}},
			},
		},
		KarmadaControlPlaneConfig: KarmadaControlPlaneConfig{
			APIServer: APIServerConfig{
				Image:            "karmada-apiserver:latest",
				AdvertiseAddress: "192.168.1.2",
				Replicas:         3,
				NodePort:         32443,
				ExtraArgs:        []Arg{{Name: "api-arg1", Value: "api-value1"}},
			},
			ControllerManager: ControllerManagerConfig{
				Image:     "karmada-controller-manager:latest",
				Replicas:  3,
				ExtraArgs: []Arg{{Name: "cm-arg1", Value: "cm-value1"}},
			},
			Scheduler: SchedulerConfig{
				Image:     "karmada-scheduler:latest",
				Replicas:  3,
				ExtraArgs: []Arg{{Name: "sched-arg1", Value: "sched-value1"}},
			},
			Webhook: WebhookConfig{
				Image:     "karmada-webhook:latest",
				Replicas:  3,
				ExtraArgs: []Arg{{Name: "webhook-arg1", Value: "webhook-value1"}},
			},
			AggregatedAPIServerConfig: AggregatedAPIServerConfig{
				Image:     "karmada-aggregated-apiserver:latest",
				Replicas:  3,
				ExtraArgs: []Arg{{Name: "aggregated-arg1", Value: "aggregated-value1"}},
			},
			KubeControllerManagerConfig: KubeControllerManagerConfig{
				Image:     "kube-controller-manager:latest",
				Replicas:  3,
				ExtraArgs: []Arg{{Name: "kcm-arg1", Value: "kcm-value1"}},
			},
			DataPath:          "/etc/karmada",
			PkiPath:           "/etc/karmada/pki",
			CRDs:              "/etc/karmada/crds",
			HostClusterDomain: "cluster.local",
		},
		ImageConfig: ImageConfig{
			KubeImageTag:           "v1.21.0",
			KubeImageRegistry:      "registry.cn-hangzhou.aliyuncs.com/google_containers",
			KubeImageMirrorCountry: "cn",
			ImagePullPolicy:        "IfNotPresent",
			ImagePullSecrets:       []string{"PullSecret1", "PullSecret2"},
		},
	}
	expectedConfig.Kind = "InitConfiguration"
	expectedConfig.APIVersion = "config.karmada.io/v1beta1"

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
