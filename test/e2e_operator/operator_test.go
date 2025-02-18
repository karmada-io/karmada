package e2e_operator

import (
	"context"
	"fmt"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	operator "github.com/karmada-io/karmada/operator/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/test/e2e/framework"
	"github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("operator testing", func() {
	homeDir := os.Getenv("HOME")
	kubeConfigPath := fmt.Sprintf("%s/.kube/%s.config", homeDir, "karmada-host")
	restConfig, err := framework.LoadRESTClientConfig(kubeConfigPath, "karmada-host")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	operatorClient, _ := operator.NewForConfig(restConfig)

	ginkgo.BeforeEach(func() {
	})

	ginkgo.Context("[operator]Test namespaced resource: deployment", func() {
		var deploymentNamespace, deploymentName string

		ginkgo.BeforeEach(func() {
			deploymentNamespace = fmt.Sprintf("karmadatest-%s", rand.String(RandomStrLength))
			deploymentName = "test" + rand.String(RandomStrLength)
		})

		ginkgo.AfterEach(func() {
			framework.RemoveNamespace(kubeClient, deploymentNamespace)
		})

		ginkgo.It("[operator]test", func() {

			ginkgo.By(fmt.Sprintf("Creating karmada %s with namespace %s ", deploymentName, deploymentNamespace), func() {
				deploymentNamespaceObj := helper.NewNamespace(deploymentNamespace)
				framework.CreateNamespace(kubeClient, deploymentNamespaceObj)
				karmada := &operatorv1alpha1.Karmada{}
				_, err := operatorClient.OperatorV1alpha1().Karmadas("default").Create(context.TODO(), karmada, v1.CreateOptions{})
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})

	})

})
