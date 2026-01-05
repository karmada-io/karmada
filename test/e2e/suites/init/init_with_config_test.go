/*
Copyright 2026 The Karmada Authors.

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

package init

import (
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/e2e/framework/resource/cmdinit"
)

const configTemplate = `
apiVersion: config.karmada.io/v1alpha1
kind: KarmadaInitConfig
spec:
  hostCluster:
    kubeconfig: "{{ .KubeconfigPath }}"
  etcd:
    local:
      dataPath: "{{ .EtcdDataPath }}"
  components:
    karmadaControllerManager:
      repository: "{{ .Registry }}/karmada-controller-manager"
      tag: "{{ .Version }}"
    karmadaScheduler:
      repository: "{{ .Registry }}/karmada-scheduler"
      tag: "{{ .Version }}"
    karmadaWebhook:
      repository: "{{ .Registry }}/karmada-webhook"
      tag: "{{ .Version }}"
    karmadaAggregatedAPIServer:
      repository: "{{ .Registry }}/karmada-aggregated-apiserver"
      tag: "{{ .Version }}"
    karmadaAPIServer:
      networking:
        port: {{ .KarmadaAPIServerNodePort }}
  karmadaDataPath: "{{ .KarmadaDataPath }}"
  karmadaPkiPath: "{{ .KarmadaPkiPath }}"
  karmadaCrds: "{{ .KarmadaCrds }}"
`

var _ = ginkgo.Describe("Custom E2E: initialize karmada control plane with config file", func() {
	var tempPki string
	var etcdDataPath string

	var configFilePath string

	ginkgo.Context("Karmadactl init with config file testing", func() {
		ginkgo.BeforeEach(func() {
			tempPki = filepath.Join(karmadaDataPath, "pki")
			etcdDataPath = filepath.Join(karmadaDataPath, "etcd-data")

			configFilePath = filepath.Join(karmadaConfigPath, "karmada-config.yaml")
		})

		ginkgo.It("Deploy karmada control plane with config file", func() {
			ginkgo.By("Generate karmada init config file with template", func() {
				renderedContent, err := util.ParseTemplate(configTemplate, struct {
					KubeconfigPath           string
					Registry                 string
					Version                  string
					KarmadaDataPath          string
					KarmadaPkiPath           string
					KarmadaCrds              string
					EtcdDataPath             string
					KarmadaAPIServerNodePort string
				}{
					KubeconfigPath:           kubeconfig,
					Registry:                 registry,
					Version:                  version,
					KarmadaDataPath:          karmadaDataPath,
					KarmadaPkiPath:           tempPki,
					KarmadaCrds:              crdsPath,
					EtcdDataPath:             etcdDataPath,
					KarmadaAPIServerNodePort: karmadaAPIServerNodePort,
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = os.MkdirAll(karmadaConfigPath, os.FileMode(0700))
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				err = os.WriteFile(configFilePath, renderedContent, os.FileMode(0600))
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("Execute the command karmadactl init", func() {
				args := []string{"init",
					"--config", configFilePath,
					"--v", "4",
				}

				cmd := framework.NewKarmadactlCommand(
					"", // kubeconfig is not specified as it will use the kubeconfig specified in karmada config file
					"",
					karmadactlPath,
					testNamespace,
					KarmadactlInitTimeOut,
					args...,
				)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.By("Waiting for components to be ready", func() {
				cmdinit.WaitAllKarmadaComponentReady(hostClient, testNamespace)
			})
		})
	})
})
