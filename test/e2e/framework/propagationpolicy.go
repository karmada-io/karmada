/*
Copyright 2021 The Karmada Authors.

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

package framework

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// RemovePropagationPolicyIfExist delete PropagationPolicy if it exists with karmada client.
func RemovePropagationPolicyIfExist(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing PropagationPolicy(%s/%s) if it exists", namespace, name), func() {
		_, err := client.PolicyV1alpha1().PropagationPolicies(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		}

		err = client.PolicyV1alpha1().PropagationPolicies(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
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

// UpdatePropagationPolicyWithSpec update PropagationSpec with karmada client.
func UpdatePropagationPolicyWithSpec(client karmada.Interface, namespace, name string, policySpec policyv1alpha1.PropagationSpec) {
	ginkgo.By(fmt.Sprintf("Updating PropagationPolicy(%s/%s) spec", namespace, name), func() {
		newPolicy, err := client.PolicyV1alpha1().PropagationPolicies(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		newPolicy.Spec = policySpec
		_, err = client.PolicyV1alpha1().PropagationPolicies(newPolicy.Namespace).Update(context.TODO(), newPolicy, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitPropagationPolicyFitWith wait PropagationPolicy sync with fit func.
func WaitPropagationPolicyFitWith(client karmada.Interface, namespace, name string, fit func(policy *policyv1alpha1.PropagationPolicy) bool) {
	gomega.Eventually(func() bool {
		policy, err := client.PolicyV1alpha1().PropagationPolicies(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(policy)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}
