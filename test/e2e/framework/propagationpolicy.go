package framework

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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

// PatchPropagationPolicy patch PropagationPolicy with karmada client.
func PatchPropagationPolicy(client karmada.Interface, namespace, name string, patch []map[string]interface{}, patchType types.PatchType) {
	ginkgo.By(fmt.Sprintf("Patching PropagationPolicy(%s/%s)", namespace, name), func() {
		patchBytes, err := json.Marshal(patch)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.PolicyV1alpha1().PropagationPolicies(namespace).Patch(context.TODO(), name, patchType, patchBytes, metav1.PatchOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdatePropagationPolicy update PropagationPolicy resourceSelectors with karmada client.
func UpdatePropagationPolicy(client karmada.Interface, namespace, name string, resourceSelectors []policyv1alpha1.ResourceSelector) {
	ginkgo.By(fmt.Sprintf("Updating PropagationPolicy(%s/%s)", namespace, name), func() {
		newPolicy, err := client.PolicyV1alpha1().PropagationPolicies(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		newPolicy.Spec.ResourceSelectors = resourceSelectors
		_, err = client.PolicyV1alpha1().PropagationPolicies(namespace).Update(context.TODO(), newPolicy, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
