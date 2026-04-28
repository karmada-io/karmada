/*
Copyright 2024 The Karmada Authors.

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
	"slices"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

// WaitEventFitWith wait PropagationPolicy sync with fit func.
func WaitEventFitWith(kubeClient kubernetes.Interface, namespace string, involvedObj string, fit func(policy corev1.Event) bool) {
	gomega.Eventually(func() bool {
		eventList, err := kubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("involvedObject.name", involvedObj).String(),
		})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		return slices.ContainsFunc(eventList.Items, fit)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}
