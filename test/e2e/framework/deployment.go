package framework

import (
	"context"
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
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

// WaitDeploymentPresentOnClusterFitWith wait deployment present on member clusters sync with fit func.
func WaitDeploymentPresentOnClusterFitWith(cluster, namespace, name string, fit func(deployment *appsv1.Deployment) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for deployment(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		dep, err := clusterClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(dep)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitDeploymentPresentOnClustersFitWith wait deployment present on cluster sync with fit func.
func WaitDeploymentPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(deployment *appsv1.Deployment) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for deployment(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitDeploymentPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitDeploymentDisappearOnCluster wait deployment disappear on cluster until timeout.
func WaitDeploymentDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for deployment(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get deployment(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitDeploymentDisappearOnClusters wait deployment disappear on member clusters until timeout.
func WaitDeploymentDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if deployment(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitDeploymentDisappearOnCluster(clusterName, namespace, name)
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

// UpdateDeploymentAnnotations update deployment's annotations.
func UpdateDeploymentAnnotations(client kubernetes.Interface, deployment *appsv1.Deployment, annotations map[string]string) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s annotations to %v", deployment.Namespace, deployment.Name, annotations), func() {
		deployment.Annotations = annotations
		gomega.Eventually(func() error {
			_, err := client.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
			return err
		}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateDeploymentVolumes update Deployment's volumes.
func UpdateDeploymentVolumes(client kubernetes.Interface, deployment *appsv1.Deployment, volumes []corev1.Volume) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s volumes", deployment.Namespace, deployment.Name), func() {
		deployment.Spec.Template.Spec.Volumes = volumes
		gomega.Eventually(func() error {
			_, err := client.AppsV1().Deployments(deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
			return err
		}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// ExtractTargetClustersFrom extract the target cluster names from deployment's related resourceBinding Information.
func ExtractTargetClustersFrom(c client.Client, deployment *appsv1.Deployment) []string {
	bindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
	binding := &workv1alpha2.ResourceBinding{}
	gomega.Eventually(func(g gomega.Gomega) (bool, error) {
		err := c.Get(context.TODO(), client.ObjectKey{Namespace: deployment.Namespace, Name: bindingName}, binding)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		if !helper.IsBindingScheduled(&binding.Status) {
			klog.Infof("The ResourceBinding(%s/%s) hasn't been scheduled.", binding.Namespace, binding.Name)
			return false, nil
		}
		return true, nil
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))

	targetClusterNames := make([]string, 0, len(binding.Spec.Clusters))
	for _, cluster := range binding.Spec.Clusters {
		targetClusterNames = append(targetClusterNames, cluster.Name)
	}
	klog.Infof("The ResourceBinding(%s/%s) schedule result is: %s", binding.Namespace, binding.Name, strings.Join(targetClusterNames, ","))
	return targetClusterNames
}

// CheckDeploymentReadyStatus check the deployment status By checking the replicas
func CheckDeploymentReadyStatus(deployment *appsv1.Deployment, wantedReplicas int32) bool {
	if deployment.Status.ReadyReplicas == wantedReplicas &&
		deployment.Status.AvailableReplicas == wantedReplicas &&
		deployment.Status.UpdatedReplicas == wantedReplicas &&
		deployment.Status.Replicas == wantedReplicas {
		return true
	}
	return false
}
