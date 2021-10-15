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

// CreateClusterPropagationPolicy create ClusterPropagationPolicy with karmada client.
func CreateClusterPropagationPolicy(client karmada.Interface, policy *policyv1alpha1.ClusterPropagationPolicy) {
	ginkgo.By(fmt.Sprintf("Creating ClusterPropagationPolicy(%s)", policy.Name), func() {
		_, err := client.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), policy, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveClusterPropagationPolicy delete ClusterPropagationPolicy with karmada client.
func RemoveClusterPropagationPolicy(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing ClusterPropagationPolicy(%s)", name), func() {
		err := client.PolicyV1alpha1().ClusterPropagationPolicies().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
