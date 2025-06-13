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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// UpdateDeploymentPaused update deployment's paused.
func UpdateDeploymentPaused(client kubernetes.Interface, deployment *appsv1.Deployment, paused bool) {
	ginkgo.By(fmt.Sprintf("Update Deployment(%s/%s)", deployment.Namespace, deployment.Name), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			deploy.Spec.Paused = paused
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateDeploymentWith update deployment with the given mutate function.
func UpdateDeploymentWith(client kubernetes.Interface, namespace, name string, mutateFunc func(deploy *appsv1.Deployment)) {
	ginkgo.By(fmt.Sprintf("Update Deployment(%s/%s)", namespace, name), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			deployCopy := deploy.DeepCopy()
			mutateFunc(deployCopy)
			if reflect.DeepEqual(deploy, deployCopy) {
				return nil
			}
			_, err = client.AppsV1().Deployments(namespace).Update(context.TODO(), deployCopy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
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
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitDeploymentFitWith wait deployment sync with fit func.
func WaitDeploymentFitWith(client kubernetes.Interface, namespace, name string, fit func(deployment *appsv1.Deployment) bool) {
	gomega.Eventually(func() bool {
		dep, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(dep)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitDeploymentPresentOnClustersFitWith wait deployment present on cluster sync with fit func.
func WaitDeploymentPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(deployment *appsv1.Deployment) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for deployment(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitDeploymentPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitDeploymentStatus wait the deployment on the cluster to have the specified replicas
func WaitDeploymentStatus(client kubernetes.Interface, deployment *appsv1.Deployment, replicas int32) {
	ginkgo.By(fmt.Sprintf("Waiting for deployment(%s/%s) status to have %d replicas", deployment.Namespace, deployment.Name, replicas), func() {
		gomega.Eventually(func() bool {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return CheckDeploymentReadyStatus(deploy, replicas)
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}

// WaitDeploymentDisappearOnCluster wait deployment disappear on cluster until timeout.
func WaitDeploymentDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for deployment(%s/%s) disappears on cluster(%s)", namespace, name, cluster)
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
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}

// WaitDeploymentDisappearOnClusters wait deployment disappear on member clusters until timeout.
func WaitDeploymentDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if deployment(%s/%s) disappears on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitDeploymentDisappearOnCluster(clusterName, namespace, name)
		}
	})
}

// UpdateDeploymentReplicas update deployment's replicas.
func UpdateDeploymentReplicas(client kubernetes.Interface, deployment *appsv1.Deployment, replicas int32) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s replicas to %d", deployment.Namespace, deployment.Name, replicas), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			deploy.Spec.Replicas = &replicas
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateDeploymentAnnotations update deployment's annotations.
func UpdateDeploymentAnnotations(client kubernetes.Interface, deployment *appsv1.Deployment, annotations map[string]string) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s annotations to %v", deployment.Namespace, deployment.Name, annotations), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			deploy.Annotations = annotations
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// AppendDeploymentAnnotations append deployment's annotations.
func AppendDeploymentAnnotations(client kubernetes.Interface, deployment *appsv1.Deployment, annotations map[string]string) {
	ginkgo.By(fmt.Sprintf("Appending Deployment(%s/%s)'s annotations to %v", deployment.Namespace, deployment.Name, annotations), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if deploy.Annotations == nil {
				deploy.Annotations = make(map[string]string, 0)
			}
			for k, v := range annotations {
				deploy.Annotations[k] = v
			}
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateDeploymentLabels update deployment's labels.
func UpdateDeploymentLabels(client kubernetes.Interface, deployment *appsv1.Deployment, labels map[string]string) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s labels to %v", deployment.Namespace, deployment.Name, labels), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			deploy.Labels = labels
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateDeploymentVolumes update deployment's volumes.
func UpdateDeploymentVolumes(client kubernetes.Interface, deployment *appsv1.Deployment, volumes []corev1.Volume) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s volumes", deployment.Namespace, deployment.Name), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			deploy.Spec.Template.Spec.Volumes = volumes
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateDeploymentServiceAccountName update deployment's serviceAccountName.
func UpdateDeploymentServiceAccountName(client kubernetes.Interface, deployment *appsv1.Deployment, serviceAccountName string) {
	ginkgo.By(fmt.Sprintf("Updating Deployment(%s/%s)'s serviceAccountName", deployment.Namespace, deployment.Name), func() {
		gomega.Eventually(func() error {
			deploy, err := client.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			deploy.Spec.Template.Spec.ServiceAccountName = serviceAccountName
			_, err = client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
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

// WaitDeploymentGetByClientFitWith wait deployment get by client fit with func.
func WaitDeploymentGetByClientFitWith(client kubernetes.Interface, namespace, name string, fit func(deployment *appsv1.Deployment) bool) {
	ginkgo.By(fmt.Sprintf("Check deployment(%s/%s) labels fit with function", namespace, name), func() {
		gomega.Eventually(func() bool {
			dep, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return fit(dep)
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}

// WaitDeploymentReplicasFitWith wait deployment replicas get by client fit with expected replicas.
func WaitDeploymentReplicasFitWith(clusters []string, namespace, name string, expectReplicas int) {
	ginkgo.By(fmt.Sprintf("Check deployment(%s/%s) replicas fit with expecting", namespace, name), func() {
		gomega.Eventually(func() bool {
			totalReplicas := 0
			for _, cluster := range clusters {
				clusterClient := GetClusterClient(cluster)
				if clusterClient == nil {
					continue
				}

				dep, err := clusterClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
				if err != nil {
					continue
				}
				totalReplicas += int(*dep.Spec.Replicas)
			}
			klog.Infof("The total replicas of deployment(%s/%s) is %d", namespace, name, totalReplicas)
			return totalReplicas == expectReplicas
		}, PollTimeout, PollInterval).Should(gomega.Equal(true))
	})
}
