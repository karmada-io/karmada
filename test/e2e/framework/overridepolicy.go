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

// CreateOverridePolicy create OverridePolicy with karmada client.
func CreateOverridePolicy(client karmada.Interface, policy *policyv1alpha1.OverridePolicy) {
	ginkgo.By(fmt.Sprintf("Creating OverridePolicy(%s/%s)", policy.Namespace, policy.Name), func() {
		_, err := client.PolicyV1alpha1().OverridePolicies(policy.Namespace).Create(context.TODO(), policy, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveOverridePolicy delete OverridePolicy with karmada client.
func RemoveOverridePolicy(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing OverridePolicy(%s/%s)", namespace, name), func() {
		err := client.PolicyV1alpha1().OverridePolicies(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
