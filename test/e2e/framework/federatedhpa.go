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

// CreateFederatedHPA create FederatedHPA with karmada client.
func CreateFederatedHPA(client karmada.Interface, fhpa *autoscalingv1alpha1.FederatedHPA) {
	ginkgo.By(fmt.Sprintf("Create FederatedHPA(%s/%s)", fhpa.Namespace, fhpa.Name), func() {
		_, err := client.AutoscalingV1alpha1().FederatedHPAs(fhpa.Namespace).Create(context.TODO(), fhpa, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveFederatedHPA delete FederatedHPA with karmada client.
func RemoveFederatedHPA(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Remove FederatedHPA(%s/%s)", namespace, name), func() {
		err := client.AutoscalingV1alpha1().FederatedHPAs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
