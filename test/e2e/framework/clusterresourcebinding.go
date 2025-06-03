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
	"strings"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// WaitClusterResourceBindingFitWith wait clusterResourceBinding fit with util timeout
func WaitClusterResourceBindingFitWith(client karmada.Interface, name string, fit func(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) bool) {
	gomega.Eventually(func() bool {
		clusterResourceBinding, err := client.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(clusterResourceBinding)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// ExtractTargetClustersFromCRB extract the target cluster names from clusterResourceBinding object.
func ExtractTargetClustersFromCRB(c client.Client, resourceKind, resourceName string) []string {
	bindingName := names.GenerateBindingName(resourceKind, resourceName)
	binding := &workv1alpha2.ClusterResourceBinding{}
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		err := c.Get(context.TODO(), client.ObjectKey{Name: bindingName}, binding)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		if !helper.IsBindingScheduled(&binding.Status) {
			klog.Infof("The ClusterResourceBinding(%s) hasn't been scheduled.", binding.Name)
			return false, nil
		}
		return true, nil
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))

	targetClusterNames := make([]string, 0, len(binding.Spec.Clusters))
	for _, cluster := range binding.Spec.Clusters {
		targetClusterNames = append(targetClusterNames, cluster.Name)
	}
	klog.Infof("The ClusterResourceBinding(%s) schedule result is: %s", binding.Name, strings.Join(targetClusterNames, ","))
	return targetClusterNames
}
