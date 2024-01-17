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

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// WaitResourceBindingFitWith wait resourceBinding fit with util timeout
func WaitResourceBindingFitWith(client karmada.Interface, namespace, name string, fit func(resourceBinding *workv1alpha2.ResourceBinding) bool) {
	gomega.Eventually(func() bool {
		resourceBinding, err := client.WorkV1alpha2().ResourceBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(resourceBinding)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}
