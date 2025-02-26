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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var workloadGVR = workloadv1alpha1.SchemeGroupVersion.WithResource("workloads")

// CreateWorkload creates Workload with dynamic client
func CreateWorkload(client dynamic.Interface, workload *workloadv1alpha1.Workload) {
	ginkgo.By(fmt.Sprintf("Creating workload(%s/%s)", workload.Namespace, workload.Name), func() {
		unstructuredObj, err := helper.ToUnstructured(workload)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.Resource(workloadGVR).Namespace(workload.Namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateWorkload updates Workload with dynamic client
func UpdateWorkload(client dynamic.Interface, workload *workloadv1alpha1.Workload, clusterName string, subresources ...string) {
	ginkgo.By(fmt.Sprintf("Update workload(%s/%s) in cluster(%s)", workload.Namespace, workload.Name, clusterName), func() {
		newUnstructuredObj, err := helper.ToUnstructured(workload)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		gomega.Eventually(func() error {
			_, err = client.Resource(workloadGVR).Namespace(workload.Namespace).Update(context.TODO(), newUnstructuredObj, metav1.UpdateOptions{}, subresources...)
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// GetWorkload gets Workload with dynamic client.
func GetWorkload(client dynamic.Interface, namespace, name string) *workloadv1alpha1.Workload {
	workload := &workloadv1alpha1.Workload{}

	ginkgo.By(fmt.Sprintf("Get workload(%s/%s)", namespace, name), func() {
		var err error
		unstructuredObj := &unstructured.Unstructured{}
		gomega.Eventually(func() error {
			unstructuredObj, err = client.Resource(workloadGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), workload)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})

	return workload
}

// RemoveWorkload deletes Workload with dynamic client.
func RemoveWorkload(client dynamic.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Remove workload(%s/%s)", namespace, name), func() {
		err := client.Resource(workloadGVR).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitWorkloadPresentOnClusterFitWith waits workload present on member cluster sync with fit func.
func WaitWorkloadPresentOnClusterFitWith(cluster, namespace, name string, fit func(workload *workloadv1alpha1.Workload) bool) {
	clusterClient := GetClusterDynamicClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for Workload(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		workload, err := clusterClient.Resource(workloadGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		typedObj := &workloadv1alpha1.Workload{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(workload.UnstructuredContent(), typedObj)
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		return fit(typedObj), nil
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitWorkloadPresentOnClustersFitWith waits workload present on member clusters sync with fit func.
func WaitWorkloadPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(workload *workloadv1alpha1.Workload) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for workload(%s/%s) synced on member clusters fit with func", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitWorkloadPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitWorkloadDisappearOnCluster waits workload disappear on cluster until timeout.
func WaitWorkloadDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterDynamicClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for workload(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.Resource(workloadGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get workload(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitWorkloadDisappearOnClusters wait workload disappear on member clusters until timeout.
func WaitWorkloadDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if workload(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitWorkloadDisappearOnCluster(clusterName, namespace, name)
		}
	})
}
