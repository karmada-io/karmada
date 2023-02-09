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

// PatchClusterPropagationPolicy patch ClusterPropagationPolicy with karmada client.
func PatchClusterPropagationPolicy(client karmada.Interface, name string, patch []map[string]interface{}, patchType types.PatchType) {
	ginkgo.By(fmt.Sprintf("Patching ClusterPropagationPolicy(%s)", name), func() {
		patchBytes, err := json.Marshal(patch)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.PolicyV1alpha1().ClusterPropagationPolicies().Patch(context.TODO(), name, patchType, patchBytes, metav1.PatchOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateClusterPropagationPolicy update ClusterPropagationPolicy resourceSelectors with karmada client.
func UpdateClusterPropagationPolicy(client karmada.Interface, name string, resourceSelectors []policyv1alpha1.ResourceSelector) {
	ginkgo.By(fmt.Sprintf("Updating ClusterPropagationPolicy(%s)", name), func() {
		newPolicy, err := client.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		newPolicy.Spec.ResourceSelectors = resourceSelectors
		_, err = client.PolicyV1alpha1().ClusterPropagationPolicies().Update(context.TODO(), newPolicy, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
