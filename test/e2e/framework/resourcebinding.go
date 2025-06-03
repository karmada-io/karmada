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
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// WaitResourceBindingFitWith wait resourceBinding fit with util timeout
func WaitResourceBindingFitWith(client karmada.Interface, namespace, name string, fit func(resourceBinding *workv1alpha2.ResourceBinding) bool) {
	gomega.Eventually(func() bool {
		resourceBinding, err := client.WorkV1alpha2().ResourceBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(resourceBinding)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// AssertBindingScheduledClusters wait deployment present on member clusters sync with fit func.
// @expectedResults contains multiple possible results about expected clusters.
func AssertBindingScheduledClusters(client karmada.Interface, namespace, name string, expectedResults [][]string) {
	ginkgo.By(fmt.Sprintf("Check ResourceBinding(%s/%s)'s target clusters is as expected", namespace, name), func() {
		gomega.Eventually(func() error {
			binding, err := client.WorkV1alpha2().ResourceBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			scheduledClusters := make([]string, 0, len(binding.Spec.Clusters))
			for _, scheduledCluster := range binding.Spec.Clusters {
				scheduledClusters = append(scheduledClusters, scheduledCluster.Name)
			}
			sort.Strings(scheduledClusters)
			for _, expectedClusters := range expectedResults {
				if reflect.DeepEqual(scheduledClusters, expectedClusters) {
					return nil
				}
			}
			return fmt.Errorf("scheduled clusters: %+v, expected possible results: %+v", scheduledClusters, expectedResults)
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitGracefulEvictionTasksDone wait GracefulEvictionTasks of the binding done.
func WaitGracefulEvictionTasksDone(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check ResourceBinding(%s/%s)'s GracefulEvictionTasks has been done", namespace, name), func() {
		gomega.Eventually(func() error {
			binding, err := client.WorkV1alpha2().ResourceBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if len(binding.Spec.GracefulEvictionTasks) > 0 {
				return fmt.Errorf("%d GracefulEvictionTasks is being processing", len(binding.Spec.GracefulEvictionTasks))
			}
			return nil
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// ExtractTargetClustersFromRB extract the target cluster names from resourceBinding object.
func ExtractTargetClustersFromRB(c client.Client, resourceKind, resourceNs, resourceName string) []string {
	bindingName := names.GenerateBindingName(resourceKind, resourceName)
	binding := &workv1alpha2.ResourceBinding{}
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		err := c.Get(context.TODO(), client.ObjectKey{Namespace: resourceNs, Name: bindingName}, binding)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		if !helper.IsBindingScheduled(&binding.Status) {
			klog.Infof("The ResourceBinding(%s/%s) hasn't been scheduled.", binding.Namespace, binding.Name)
			return false, nil
		}
		return true, nil
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))

	targetClusterNames := make([]string, 0, len(binding.Spec.Clusters))
	for _, cluster := range binding.Spec.Clusters {
		targetClusterNames = append(targetClusterNames, cluster.Name)
	}
	klog.Infof("The ResourceBinding(%s/%s) schedule result is: %s", binding.Namespace, binding.Name, strings.Join(targetClusterNames, ","))
	return targetClusterNames
}
