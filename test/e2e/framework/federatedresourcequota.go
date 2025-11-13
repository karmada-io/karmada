/*
Copyright 2022 The Karmada Authors.

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
	"reflect"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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

// UpdateFederatedResourceQuotaWithPatch update FederatedResourceQuota with patch bytes.
func UpdateFederatedResourceQuotaWithPatch(client karmada.Interface, namespace, name string, patch []map[string]interface{}, patchType types.PatchType) {
	ginkgo.By(fmt.Sprintf("Updating FederatedResourceQuota(%s/%s)", namespace, name), func() {
		bytes, err := json.Marshal(patch)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.PolicyV1alpha1().FederatedResourceQuotas(namespace).Patch(context.TODO(), name, patchType, bytes, metav1.PatchOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitFederatedResourceQuotaCollectStatus wait FederatedResourceQuota collect status successfully.
func WaitFederatedResourceQuotaCollectStatus(client karmada.Interface, namespace, name string) {
	ginkgo.By("wait status collect correctly", func() {
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			frq, err := client.PolicyV1alpha1().FederatedResourceQuotas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			g.Expect(err).NotTo(gomega.HaveOccurred())

			staticAssignments := frq.Spec.StaticAssignments
			aggregatedStatus := frq.Status.AggregatedStatus
			for _, assign := range staticAssignments {
				matched := false
				for _, aggregated := range aggregatedStatus {
					if assign.ClusterName != aggregated.ClusterName {
						continue
					}

					matched = true
					if reflect.DeepEqual(assign.Hard, aggregated.Hard) {
						break
					}
					return false, nil
				}
				if !matched {
					return false, nil
				}
			}

			return true, nil
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}
