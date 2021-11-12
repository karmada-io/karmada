/*
Copyright The Karmada Authors.

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
