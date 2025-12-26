/*
Copyright 2025 The Karmada Authors.

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
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/e2e/framework/resource/cmdinit"
)

const (
	KarmadactlInitTimeOut = time.Minute * 10
)

const (
	KarmadaEtcdFlag              = "--etcd-extra-args"
	KarmadaControllerManagerFlag = "--karmada-controller-manager-extra-args"
)

var karmadaEtcdExpectedExtraArgs = []string{
	"--snapshot-count=5000",    // Override default parameters in the format --key=value.
	"--heartbeat-interval=100", // Additional parameters are in the format --key=value.
}

var karmadaControllerManagerExpectedExtraArgs = []string{
	"--v=2",          // Override default parameters in the format --key=value.
	"--enable-pprof", // The format for additional parameters is --key.
	"--skipped-propagating-namespaces=kube-system,default,my-ns", // Additional parameters are in the format --key=value1,value2.
}

var _ = ginkgo.Describe("Component command line flags testing", func() {
	ginkgo.Context("Customize karmada components command line flags", func() {
		var tempPki string
		var etcdDataPath string

		ginkgo.BeforeEach(func() {
			tempPki = filepath.Join(karmadaDataPath, "pki")
			etcdDataPath = filepath.Join(karmadaDataPath, "etcd-data")
		})

		ginkgo.It("Customize karmada components command line flags via karmadactl init command flags", func() {
			// step1 Execute the Karmadactl init command.
			ginkgo.By("Execute the command karmadactl init", func() {
				args := []string{"init",
					"--karmada-data", karmadaDataPath,
					"--karmada-pki", tempPki,
					"--crds", crdsPath,
					"--karmada-aggregated-apiserver-image", karmadaAggregatedAPIServerImage,
					"--karmada-controller-manager-image", karmadaControllerManagerImage,
					"--karmada-scheduler-image", karmadaSchedulerImage,
					"--karmada-webhook-image", karmadaWebhookImage,
					"--port", karmadaAPIServerNodePort,
					"--etcd-data", etcdDataPath,
					"--v", "4",
					// Here we select two specific examples: etcd in StatefulSet and karmada-controller-manager in Deployment.
					// StatefulSet etcd
					KarmadaEtcdFlag, strings.Join(karmadaEtcdExpectedExtraArgs, ","),
					// Deployment karmada-controller-manager
					KarmadaControllerManagerFlag, strings.Join(karmadaControllerManagerExpectedExtraArgs, ","),
				}

				cmd := framework.NewKarmadactlCommand(
					kubeconfig,
					"",
					karmadactlPath,
					testNamespace,
					KarmadactlInitTimeOut,
					args...,
				)
				_, err := cmd.ExecOrDie()
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			// step2 Waiting for components to be ready.
			ginkgo.By("Waiting for components to be ready.", func() {
				cmdinit.WaitAllKarmadaComponentReady(hostClient, testNamespace)
			})

			// step3.1 Check Command Line Flags
			ginkgo.By(fmt.Sprintf("Checking command line flags for %s", cmdinit.KarmadaEtcdName), func() {
				// 1. Get StatefulSet
				statefulSet, err := hostClient.AppsV1().StatefulSets(testNamespace).
					Get(context.TODO(), cmdinit.KarmadaEtcdName, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Get statefulset(%s/%s) failed, err: %v", testNamespace, cmdinit.KarmadaEtcdName, err)
					return
				}

				// 2. Get command line flags
				actualCommands, err := cmdinit.GetComponentCommandLineFlags(cmdinit.KarmadaEtcdName, statefulSet.Spec.Template)
				if err != nil {
					klog.Errorf("%v", err)
					return
				}

				// 3. check etcd command line flags
				if validateComponentExtraArgs(actualCommands, karmadaEtcdExpectedExtraArgs) {
					klog.Infof("%s command line flags validation passed", cmdinit.KarmadaEtcdName)
					return
				}
				klog.Errorf("%s command line flags validation failed. The actual commands are: %v", cmdinit.KarmadaEtcdName, strings.Join(actualCommands, "\n"))
			})

			// step3.2 Check Command Line Flags
			ginkgo.By(fmt.Sprintf("Checking command line flags for %s", cmdinit.KarmadaControllerManagerName), func() {
				// 1. Get Deployment
				deployment, err := hostClient.AppsV1().Deployments(testNamespace).
					Get(context.TODO(), cmdinit.KarmadaControllerManagerName, metav1.GetOptions{})
				if err != nil {
					klog.Errorf("Get deployment(%s/%s) failed, err: %v", testNamespace, cmdinit.KarmadaControllerManagerName, err)
					return
				}

				// 2. Get command line flags
				actualCommands, err := cmdinit.GetComponentCommandLineFlags(cmdinit.KarmadaControllerManagerName, deployment.Spec.Template)
				if err != nil {
					klog.Errorf("%v", err)
					return
				}

				// 3. check karmada-controller-manager command line flags
				if validateComponentExtraArgs(actualCommands, karmadaControllerManagerExpectedExtraArgs) {
					klog.Infof("%s command line flags validation passed", cmdinit.KarmadaControllerManagerName)
					return
				}
				klog.Errorf("%s command line flags validation failed. The actual commands are: %v", cmdinit.KarmadaEtcdName, strings.Join(actualCommands, "\n"))
			})
		})
	})
})

// tools
func validateComponentExtraArgs(actual, expected []string) bool {
	flags := make(map[string]bool, len(expected))

	for _, arg := range expected {
		flags[arg] = false
	}

	for _, command := range actual {
		if _, ok := flags[command]; ok {
			flags[command] = true
		}
	}

	for _, flag := range flags {
		if !flag {
			return false
		}
	}

	return true
}
