/*
Copyright 2023 The Karmada Authors.
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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateCronFederatedHPA create CronFederatedHPA with karmada client.
func CreateCronFederatedHPA(client karmada.Interface, fhpa *autoscalingv1alpha1.CronFederatedHPA) {
	ginkgo.By(fmt.Sprintf("Create FederatedHPA(%s/%s)", fhpa.Namespace, fhpa.Name), func() {
		_, err := client.AutoscalingV1alpha1().CronFederatedHPAs(fhpa.Namespace).Create(context.TODO(), fhpa, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveCronFederatedHPA delete CronFederatedHPA with karmada client.
func RemoveCronFederatedHPA(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Remove FederatedHPA(%s/%s)", namespace, name), func() {
		err := client.AutoscalingV1alpha1().CronFederatedHPAs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateCronFederatedHPAWithRule update CronFederatedHPA with karmada client.
func UpdateCronFederatedHPAWithRule(client karmada.Interface, namespace, name string, rule []autoscalingv1alpha1.CronFederatedHPARule) {
	ginkgo.By(fmt.Sprintf("Updating CronFederatedHPA(%s/%s)", namespace, name), func() {
		newCronFederatedHPA, err := client.AutoscalingV1alpha1().CronFederatedHPAs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		newCronFederatedHPA.Spec.Rules = rule
		_, err = client.AutoscalingV1alpha1().CronFederatedHPAs(namespace).Update(context.TODO(), newCronFederatedHPA, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
