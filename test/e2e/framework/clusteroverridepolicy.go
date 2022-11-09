package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateClusterOverridePolicy create ClusterOverridePolicy with karmada client.
func CreateClusterOverridePolicy(client karmada.Interface, policy *policyv1alpha1.ClusterOverridePolicy) {
	ginkgo.By(fmt.Sprintf("Creating ClusterOverridePolicy(%s)", policy.Name), func() {
		_, err := client.PolicyV1alpha1().ClusterOverridePolicies().Create(context.TODO(), policy, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveClusterOverridePolicy delete ClusterOverridePolicy with karmada client.
func RemoveClusterOverridePolicy(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing ClusterOverridePolicy(%s)", name), func() {
		err := client.PolicyV1alpha1().ClusterOverridePolicies().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
