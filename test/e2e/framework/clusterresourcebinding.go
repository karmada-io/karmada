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

// WaitClusterResourceBindingFitWith wait clusterResourceBinding fit with util timeout
func WaitClusterResourceBindingFitWith(client karmada.Interface, name string, fit func(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) bool) {
	gomega.Eventually(func() bool {
		clusterResourceBinding, err := client.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(clusterResourceBinding)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}
