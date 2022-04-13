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
)

// CreateClusterRole create clusterRole.
func CreateClusterRole(client kubernetes.Interface, clusterRole *rbacv1.ClusterRole) {
	ginkgo.By(fmt.Sprintf("Creating ClusterRole(%s)", clusterRole.Name), func() {
		_, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// CreateClusterRoleBinding create clusterRoleBinding.
func CreateClusterRoleBinding(client kubernetes.Interface, clusterRoleBinding *rbacv1.ClusterRoleBinding) {
	ginkgo.By(fmt.Sprintf("Creating ClusterRoleBinding(%s)", clusterRoleBinding.Name), func() {
		_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
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
