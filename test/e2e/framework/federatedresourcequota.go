package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateFederatedResourceQuota create FederatedResourceQuota with karmada client.
func CreateFederatedResourceQuota(client karmada.Interface, federatedResourceQuota *policyv1alpha1.FederatedResourceQuota) {
	ginkgo.By(fmt.Sprintf("Creating FederatedResourceQuota(%s/%s)", federatedResourceQuota.Namespace, federatedResourceQuota.Name), func() {
		_, err := client.PolicyV1alpha1().FederatedResourceQuotas(federatedResourceQuota.Namespace).Create(context.TODO(), federatedResourceQuota, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveFederatedResourceQuota delete FederatedResourceQuota with karmada client.
func RemoveFederatedResourceQuota(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing FederatedResourceQuota(%s/%s)", namespace, name), func() {
		err := client.PolicyV1alpha1().FederatedResourceQuotas(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
