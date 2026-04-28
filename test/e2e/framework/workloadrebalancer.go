/*
Copyright 2021 The Karmada Authors.

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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateWorkloadRebalancer create WorkloadRebalancer with karmada client.
func CreateWorkloadRebalancer(client karmada.Interface, rebalancer *appsv1alpha1.WorkloadRebalancer) {
	ginkgo.By(fmt.Sprintf("Creating WorkloadRebalancer(%s)", rebalancer.Name), func() {
		newRebalancer, err := client.AppsV1alpha1().WorkloadRebalancers().Create(context.TODO(), rebalancer, metav1.CreateOptions{})
		*rebalancer = *newRebalancer
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveWorkloadRebalancer delete WorkloadRebalancer.
func RemoveWorkloadRebalancer(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing WorkloadRebalancer(%s)", name), func() {
		err := client.AppsV1alpha1().WorkloadRebalancers().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateWorkloadRebalancer updates WorkloadRebalancer with karmada client.
// if workloads/ttl is a nil pointer, keep previous value unchanged.
func UpdateWorkloadRebalancer(client karmada.Interface, name string, workloads *[]appsv1alpha1.ObjectReference, ttl *int32) {
	ginkgo.By(fmt.Sprintf("Updating WorkloadRebalancer(%s)'s workloads", name), func() {
		gomega.Eventually(func() error {
			rebalancer, err := client.AppsV1alpha1().WorkloadRebalancers().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if workloads != nil {
				rebalancer.Spec.Workloads = *workloads
			}
			if ttl != nil {
				rebalancer.Spec.TTLSecondsAfterFinished = ttl
			}
			_, err = client.AppsV1alpha1().WorkloadRebalancers().Update(context.TODO(), rebalancer, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitRebalancerObservedWorkloads wait observedWorkloads in WorkloadRebalancer fit with util timeout
func WaitRebalancerObservedWorkloads(client karmada.Interface, name string, expectedWorkloads []appsv1alpha1.ObservedWorkload) {
	ginkgo.By(fmt.Sprintf("Waiting for WorkloadRebalancer(%s) observedWorkload match to expected result", name), func() {
		gomega.Eventually(func() error {
			rebalancer, err := client.AppsV1alpha1().WorkloadRebalancers().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(rebalancer.Status.ObservedWorkloads, expectedWorkloads) {
				return fmt.Errorf("observedWorkloads: %+v, expectedWorkloads: %+v", rebalancer.Status.ObservedWorkloads, expectedWorkloads)
			}
			return nil
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitRebalancerDisappear wait WorkloadRebalancer disappear until timeout.
func WaitRebalancerDisappear(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Waiting for WorkloadRebalancer(%s) disappears", name), func() {
		gomega.Eventually(func() error {
			rebalancer, err := client.AppsV1alpha1().WorkloadRebalancers().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			return fmt.Errorf("WorkloadRebalancer %s still exist: %+v", name, rebalancer)
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}
