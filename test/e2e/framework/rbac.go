/*
Copyright 2022 The Karmada Authors.

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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateRole create role.
func CreateRole(client kubernetes.Interface, role *rbacv1.Role) {
	ginkgo.By(fmt.Sprintf("Creating Role(%s/%s)", role.Namespace, role.Name), func() {
		_, err := client.RbacV1().Roles(role.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveRole delete role.
func RemoveRole(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Remove Role(%s/%s)", namespace, name), func() {
		err := client.RbacV1().Roles(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitRolePresentOnClustersFitWith wait role present on clusters sync with fit func.
func WaitRolePresentOnClustersFitWith(clusters []string, namespace, name string, fit func(role *rbacv1.Role) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for role(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitRolePresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitRolePresentOnClusterFitWith wait role present on member cluster sync with fit func.
func WaitRolePresentOnClusterFitWith(cluster, namespace, name string, fit func(role *rbacv1.Role) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for role(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		role, err := clusterClient.RbacV1().Roles(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(role)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitRoleDisappearOnClusters wait role disappear on member clusters until timeout.
func WaitRoleDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if role(%s/%s) disappear on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitRoleDisappearOnCluster(clusterName, namespace, name)
		}
	})
}

// WaitRoleDisappearOnCluster wait role disappear on cluster until timeout.
func WaitRoleDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for role(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.RbacV1().Roles(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get role(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// CreateClusterRole create clusterRole.
func CreateClusterRole(client kubernetes.Interface, clusterRole *rbacv1.ClusterRole) {
	ginkgo.By(fmt.Sprintf("Creating ClusterRole(%s)", clusterRole.Name), func() {
		_, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveClusterRole delete clusterRole.
func RemoveClusterRole(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Remove ClusterRole(%s)", name), func() {
		err := client.RbacV1().ClusterRoles().Delete(context.TODO(), name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitClusterRolePresentOnClustersFitWith wait clusterRole present on clusters sync with fit func.
func WaitClusterRolePresentOnClustersFitWith(clusters []string, name string, fit func(clusterRole *rbacv1.ClusterRole) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for clusterRole(%s) synced on member clusters", name), func() {
		for _, clusterName := range clusters {
			WaitClusterRolePresentOnClusterFitWith(clusterName, name, fit)
		}
	})
}

// WaitClusterRolePresentOnClusterFitWith wait clusterRole present on member cluster sync with fit func.
func WaitClusterRolePresentOnClusterFitWith(cluster, name string, fit func(clusterRole *rbacv1.ClusterRole) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for clusterRole(%s) synced on cluster(%s)", name, cluster)
	gomega.Eventually(func() bool {
		clusterRole, err := clusterClient.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(clusterRole)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitClusterRoleDisappearOnClusters wait clusterRole disappear on member clusters until timeout.
func WaitClusterRoleDisappearOnClusters(clusters []string, name string) {
	ginkgo.By(fmt.Sprintf("Check if clusterRole(%s) disappear on member clusters", name), func() {
		for _, clusterName := range clusters {
			WaitClusterRoleDisappearOnCluster(clusterName, name)
		}
	})
}

// WaitClusterRoleDisappearOnCluster wait clusterRole disappear on cluster until timeout.
func WaitClusterRoleDisappearOnCluster(cluster, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for clusterRole(%s) disappear on cluster(%s)", name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get clusterRole(%s) on cluster(%s), err: %v", name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitClusterRoleGetByClientFitWith wait clusterRole get by client fit with func.
func WaitClusterRoleGetByClientFitWith(client kubernetes.Interface, name string, fit func(clusterRole *rbacv1.ClusterRole) bool) {
	ginkgo.By(fmt.Sprintf("Check clusterRole(%s) labels fit with function", name), func() {
		gomega.Eventually(func() bool {
			clusterRole, err := client.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return fit(clusterRole)
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}

// CreateRoleBinding create roleBinding.
func CreateRoleBinding(client kubernetes.Interface, roleBinding *rbacv1.RoleBinding) {
	ginkgo.By(fmt.Sprintf("Creating RoleBinding(%s/%s)", roleBinding.Namespace, roleBinding.Name), func() {
		_, err := client.RbacV1().RoleBindings(roleBinding.Namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveRoleBinding delete roleBinding.
func RemoveRoleBinding(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Remove RoleBinding(%s/%s)", namespace, name), func() {
		err := client.RbacV1().RoleBindings(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitRoleBindingPresentOnClustersFitWith wait robeBinding present on clusters sync with fit func.
func WaitRoleBindingPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(roleBinding *rbacv1.RoleBinding) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for rolebinding(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitRoleBindingPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitRoleBindingPresentOnClusterFitWith wait roleBinding present on member cluster sync with fit func.
func WaitRoleBindingPresentOnClusterFitWith(cluster, namespace, name string, fit func(roleBinding *rbacv1.RoleBinding) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for roleBinding(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		roleBinding, err := clusterClient.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(roleBinding)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitRoleBindingDisappearOnClusters wait roleBinding disappear on member clusters until timeout.
func WaitRoleBindingDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if roleBinding(%s/%s) disappear on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitRoleBindingDisappearOnCluster(clusterName, namespace, name)
		}
	})
}

// WaitRoleBindingDisappearOnCluster wait roleBinding disappear on cluster until timeout.
func WaitRoleBindingDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for roleBinding(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get roleBinding(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// CreateClusterRoleBinding create clusterRoleBinding.
func CreateClusterRoleBinding(client kubernetes.Interface, clusterRoleBinding *rbacv1.ClusterRoleBinding) {
	ginkgo.By(fmt.Sprintf("Creating ClusterRoleBinding(%s)", clusterRoleBinding.Name), func() {
		_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveClusterRoleBinding delete clusterRoleBinding.
func RemoveClusterRoleBinding(client kubernetes.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Remove ClusterRoleBinding(%s)", name), func() {
		err := client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitClusterRoleBindingPresentOnClustersFitWith wait clusterRoleBinding present on clusters sync with fit func.
func WaitClusterRoleBindingPresentOnClustersFitWith(clusters []string, name string, fit func(clusterRoleBinding *rbacv1.ClusterRoleBinding) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for clusterRolebinding(%s) synced on member clusters", name), func() {
		for _, clusterName := range clusters {
			WaitClusterRoleBindingPresentOnClusterFitWith(clusterName, name, fit)
		}
	})
}

// WaitClusterRoleBindingPresentOnClusterFitWith wait clusterRoleBinding present on member cluster sync with fit func.
func WaitClusterRoleBindingPresentOnClusterFitWith(cluster, name string, fit func(clusterRoleBinding *rbacv1.ClusterRoleBinding) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for clusterRolebinding(%s) synced on cluster(%s)", name, cluster)
	gomega.Eventually(func() bool {
		clusterRoleBinding, err := clusterClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(clusterRoleBinding)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitClusterRoleBindingDisappearOnClusters wait clusterRoleBinding disappear on member clusters until timeout.
func WaitClusterRoleBindingDisappearOnClusters(clusters []string, name string) {
	ginkgo.By(fmt.Sprintf("Check if clusterRoleBinding(%s) disappear on member clusters", name), func() {
		for _, clusterName := range clusters {
			WaitClusterRoleBindingDisappearOnCluster(clusterName, name)
		}
	})
}

// WaitClusterRoleBindingDisappearOnCluster wait clusterRoleBinding disappear on cluster until timeout.
func WaitClusterRoleBindingDisappearOnCluster(cluster, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for clusterRoleBinding(%s) disappear on cluster(%s)", name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get clusterRoleBinding(%s) on cluster(%s), err: %v", name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// CreateServiceAccount create serviceaccount.
func CreateServiceAccount(client kubernetes.Interface, serviceaccount *corev1.ServiceAccount) {
	ginkgo.By(fmt.Sprintf("Creating ServiceAccount(%s/%s)", serviceaccount.Namespace, serviceaccount.Name), func() {
		_, err := client.CoreV1().ServiceAccounts(serviceaccount.Namespace).Create(context.TODO(), serviceaccount, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveServiceAccount delete serviceaccount.
func RemoveServiceAccount(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Remove ServiceAccount(%s/%s)", namespace, name), func() {
		err := client.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitServiceAccountPresentOnClusterFitWith wait sa present on member clusters sync with fit func.
func WaitServiceAccountPresentOnClusterFitWith(cluster, namespace, name string, fit func(sa *corev1.ServiceAccount) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for serviceAccount(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		sa, err := clusterClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(sa)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitServiceAccountPresentOnClustersFitWith wait sa present on cluster sync with fit func.
func WaitServiceAccountPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(sa *corev1.ServiceAccount) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for serviceAccount(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitServiceAccountPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitServiceAccountDisappearOnCluster wait sa disappear on cluster until timeout.
func WaitServiceAccountDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for sa(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get sa(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitServiceAccountDisappearOnClusters wait sa disappear on member clusters until timeout.
func WaitServiceAccountDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if sa(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitServiceAccountDisappearOnCluster(clusterName, namespace, name)
		}
	})
}
