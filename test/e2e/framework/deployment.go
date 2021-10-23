package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateDeployment create Deployment.
func CreateDeployment(client kubernetes.Interface, deployment *appsv1.Deployment) {
	ginkgo.By(fmt.Sprintf("Creating Deployment(%s/%s)", deployment.Namespace, deployment.Name), func() {
		_, err := client.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveDeployment delete Deployment.
func RemoveDeployment(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Deployment(%s/%s)", namespace, name), func() {
		err := client.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitDeploymentPresentOnClusters wait deployment present on member clusters until timeout.
func WaitDeploymentPresentOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if deployment(%s/%s) present on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			clusterClient := GetClusterClient(clusterName)
			gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

			klog.Infof("Waiting for deployment present on cluster(%s)", clusterName)
			gomega.Eventually(func(g gomega.Gomega) (bool, error) {
				_, err := clusterClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				g.Expect(err).NotTo(gomega.HaveOccurred())

				return true, nil
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		}
	})
}

// WaitDeploymentDisappearOnClusters wait deployment disappear on member clusters until timeout.
func WaitDeploymentDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if deployment(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			clusterClient := GetClusterClient(clusterName)
			gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

			klog.Infof("Waiting for deployment disappear on cluster(%s)", clusterName)
			gomega.Eventually(func() bool {
				_, err := clusterClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		}
	})
}

// UpdateDeploymentReplicas update deployment's replicas.
func UpdateDeploymentReplicas(client kubernetes.Interface, deployment *appsv1.Deployment, replicas int32) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s replicas to %d", deployment.Namespace, deployment.Name, replicas), func() {
		deployment.Spec.Replicas = &replicas
		gomega.Eventually(func() error {
			_, err := client.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
			return err
		}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitDeploymentPresentOnClusterFitWith wait deployment present on member clusters fit with fit func.
func WaitDeploymentPresentOnClusterFitWith(client kubernetes.Interface, namespace, name string, fit func(deployment *appsv1.Deployment) bool) {
	gomega.Eventually(func() bool {
		dep, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(dep)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}
