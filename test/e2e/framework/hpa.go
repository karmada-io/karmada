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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateHPA create HPA.
func CreateHPA(client kubernetes.Interface, hpa *autoscalingv2.HorizontalPodAutoscaler) {
	ginkgo.By(fmt.Sprintf("Creating HPA(%s/%s)", hpa.Namespace, hpa.Name), func() {
		_, err := client.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveHPA delete HPA.
func RemoveHPA(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing HPA(%s/%s)", namespace, name), func() {
		err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateHPAWithMinReplicas update HPA with replicas.
func UpdateHPAWithMinReplicas(client kubernetes.Interface, namespace, name string, minReplicas int32) {
	ginkgo.By(fmt.Sprintf("Updating HPA(%s/%s)", namespace, name), func() {
		newHPA, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		newHPA.Spec.MinReplicas = &minReplicas
		_, err = client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(context.TODO(), newHPA, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
