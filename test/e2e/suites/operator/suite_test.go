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

package e2e

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	operator "github.com/karmada-io/karmada/operator/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/e2e/framework"
	operatorresource "github.com/karmada-io/karmada/test/e2e/framework/resource/operator"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	// RandomStrLength represents the random string length to combine names.
	RandomStrLength = 5
	// KarmadaInstanceNamePrefix the prefix of the karmada instance name.
	KarmadaInstanceNamePrefix = "karmadatest-"
)

var (
	// pollInterval defines the interval time for a poll operation.
	pollInterval time.Duration
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout time.Duration
)

var (
	hostContext    string
	kubeconfig     string
	karmadactlPath string
	restConfig     *rest.Config
	kubeClient     kubernetes.Interface
	testNamespace  string
	operatorClient operator.Interface
)

func init() {
	// usage ginkgo -- --poll-interval=5s --poll-timeout=5m
	// eg. ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/ -- --poll-interval=5s --poll-timeout=5m
	flag.DurationVar(&pollInterval, "poll-interval", 5*time.Second, "poll-interval defines the interval time for a poll operation")
	flag.DurationVar(&pollTimeout, "poll-timeout", 300*time.Second, "poll-timeout defines the time which the poll operation times out")
	flag.StringVar(&hostContext, "host-context", "karmada-host", "Name of the host cluster context in control plane kubeconfig file.")
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Operator Suite")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func([]byte) {
	kubeconfig = os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())

	goPathCmd := exec.Command("go", "env", "GOPATH")
	goPath, err := goPathCmd.CombinedOutput()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	formatGoPath := strings.Trim(string(goPath), "\n")
	karmadactlPath = formatGoPath + "/bin/karmadactl"
	gomega.Expect(karmadactlPath).ShouldNot(gomega.BeEmpty())

	restConfig, err = framework.LoadRESTClientConfig(kubeconfig, hostContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	testNamespace = fmt.Sprintf("operatortest-%s", rand.String(RandomStrLength))
	err = setupTestNamespace(testNamespace, kubeClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	operatorClient, err = operator.NewForConfig(restConfig)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	// cleanup all namespaces we created both in control plane and member clusters.
	// It will not return error even if there is no such namespace in there that may happen in case setup failed.
	err := cleanupTestNamespace(testNamespace, kubeClient)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}, func() {})

// setupTestNamespace will create a namespace in control plane and all member clusters, most of cases will run against it.
// The reason why we need a separated namespace is it will make it easier to cleanup resources deployed by the testing.
func setupTestNamespace(namespace string, kubeClient kubernetes.Interface) error {
	namespaceObj := helper.NewNamespace(namespace)
	_, err := util.CreateNamespace(kubeClient, namespaceObj)
	if err != nil {
		return err
	}

	return nil
}

// cleanupTestNamespace will remove the namespace we set up before for the whole testing.
func cleanupTestNamespace(namespace string, kubeClient kubernetes.Interface) error {
	err := util.DeleteNamespace(kubeClient, namespace)
	if err != nil {
		return err
	}

	return nil
}

// InitializeKarmadaInstance initializes a karmada instance.
func InitializeKarmadaInstance(client operator.Interface, namespace, name string, mutateFns ...func(karmada *operatorv1alpha1.Karmada)) {
	karmada := helper.NewKarmada(namespace, name)
	for _, mutateFn := range mutateFns {
		mutateFn(karmada)
	}
	now := time.Now()
	err := operatorresource.CreateKarmadaInstance(client, karmada)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	operatorresource.WaitKarmadaReady(client, namespace, name, now)
}
