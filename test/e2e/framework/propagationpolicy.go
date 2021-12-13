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

// CreatePropagationPolicy create PropagationPolicy with karmada client.
func CreatePropagationPolicy(client karmada.Interface, policy *policyv1alpha1.PropagationPolicy) {
	ginkgo.By(fmt.Sprintf("Creating PropagationPolicy(%s/%s)", policy.Namespace, policy.Name), func() {
		_, err := client.PolicyV1alpha1().PropagationPolicies(policy.Namespace).Create(context.TODO(), policy, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemovePropagationPolicy delete PropagationPolicy with karmada client.
func RemovePropagationPolicy(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing PropagationPolicy(%s/%s)", namespace, name), func() {
		err := client.PolicyV1alpha1().PropagationPolicies(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
