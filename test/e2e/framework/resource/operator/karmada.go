/*
Copyright 2025 The Karmada Authors.

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

package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	operator "github.com/karmada-io/karmada/operator/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/test/e2e/framework"
)

// WaitKarmadaReady wait karmada instance ready until timeout.
// Since the karmada-operator updates the `karmada.spec` first and then the `karmada.status`, in order to ensure that the `ready` condition indicates
// that the `karmada.spec` has been applied correctly, it will check the `lastTransitionTime` of the `ready` condition.
func WaitKarmadaReady(client operator.Interface, namespace, name string, lastTransitionTime time.Time) {
	klog.Infof("Waiting for karmada instance %s/%s ready", namespace, name)
	ginkgo.By(fmt.Sprintf("Waiting for karmada instance %s/%s ready", namespace, name), func() {
		gomega.Eventually(func(g gomega.Gomega) bool {
			karmada, err := client.OperatorV1alpha1().Karmadas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			for _, condition := range karmada.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" && condition.LastTransitionTime.After(lastTransitionTime) {
					return true
				}
			}
			return false
		}, framework.PollTimeout, framework.PollInterval).Should(gomega.Equal(true))
	})
}

// CreateKarmadaInstance creates a karmada instance.
func CreateKarmadaInstance(operatorClient operator.Interface, karmada *operatorv1alpha1.Karmada) error {
	_, err := operatorClient.OperatorV1alpha1().Karmadas(karmada.GetNamespace()).Create(context.TODO(), karmada, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}

		return err
	}
	return nil
}

// UpdateKarmadaInstanceWithSpec updates karmada instance with spec.
func UpdateKarmadaInstanceWithSpec(client operator.Interface, namespace, name string, karmadaSpec operatorv1alpha1.KarmadaSpec) {
	ginkgo.By(fmt.Sprintf("Updating Karmada(%s/%s) spec", namespace, name), func() {
		karmada, err := client.OperatorV1alpha1().Karmadas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		karmada.Spec = karmadaSpec
		_, err = client.OperatorV1alpha1().Karmadas(namespace).Update(context.TODO(), karmada, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// GetLastTransitionTime gets the last transition time of the condition, return time.Now() if not found.
func GetLastTransitionTime(karmada *operatorv1alpha1.Karmada, conditionType operatorv1alpha1.ConditionType) time.Time {
	for _, condition := range karmada.Status.Conditions {
		if condition.Type == string(conditionType) {
			return condition.LastTransitionTime.Time
		}
	}
	return time.Now()
}
