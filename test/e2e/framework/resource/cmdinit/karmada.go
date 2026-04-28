/*
Copyright 2025 The Karmada Authors.

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

package cmdinit

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/test/e2e/framework"
)

const (
	// KarmadaEtcdName is the name of Karmada etcd.
	KarmadaEtcdName = "etcd"
	// KarmadaAPIServerName is the name of Karmada apiserver.
	KarmadaAPIServerName = "karmada-apiserver"
	// KarmadaAggregatedAPIServerName is the name of Karmada aggregated apiserver.
	KarmadaAggregatedAPIServerName = "karmada-aggregated-apiserver"
	// KarmadaKubeControllerManagerName is the name of kube-controller-manager.
	KarmadaKubeControllerManagerName = "kube-controller-manager"
	// KarmadaSchedulerName is the name of Karmada scheduler.
	KarmadaSchedulerName = "karmada-scheduler"
	// KarmadaControllerManagerName is the name of Karmada controller manager.
	KarmadaControllerManagerName = "karmada-controller-manager"
	// KarmadaWebhookName is the name of Karmada webhook.
	KarmadaWebhookName = "karmada-webhook"
)

// WaitAllKarmadaComponentReady wait all karmada components ready
func WaitAllKarmadaComponentReady(client kubernetes.Interface, namespace string) {
	klog.Infof("Waiting for all karmada components ready (0/7)")

	// 1. etcd
	klog.Infof("Waiting for etcd ready")
	framework.WaitStatefulSetFitWith(client, namespace, KarmadaEtcdName, func(statefulSet *appsv1.StatefulSet) bool {
		return statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas
	})

	// 2. karmada-apiserver
	klog.Infof("Waiting for karmada-apiserver ready")
	framework.WaitDeploymentFitWith(client, namespace, KarmadaAPIServerName, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	})

	// 3. karmada-aggregated-apiserver
	klog.Infof("Waiting for karmada-aggregated-apiserver ready")
	framework.WaitDeploymentFitWith(client, namespace, KarmadaAggregatedAPIServerName, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	})

	// 4. kube-controller-manager
	klog.Infof("Waiting for kube-controller-manager ready")
	framework.WaitDeploymentFitWith(client, namespace, KarmadaKubeControllerManagerName, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	})

	// 5. karmada-scheduler
	klog.Infof("Waiting for karmada-scheduler ready")
	framework.WaitDeploymentFitWith(client, namespace, KarmadaSchedulerName, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	})

	// 6. karmada-controller-manager
	klog.Infof("Waiting for karmada-controller-manager ready")
	framework.WaitDeploymentFitWith(client, namespace, KarmadaControllerManagerName, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	})

	// 7. karmada-webhook
	klog.Infof("Waiting for karmada-webhook ready")
	framework.WaitDeploymentFitWith(client, namespace, KarmadaWebhookName, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
	})

	klog.Infof("All karmada components ready")
}

// GetComponentCommandLineFlags get component command line flags
func GetComponentCommandLineFlags(componentName string, podTemplateSpec corev1.PodTemplateSpec) ([]string, error) {
	if len(podTemplateSpec.Spec.Containers) == 0 {
		return nil, fmt.Errorf("no containers found in %s", componentName)
	}
	return podTemplateSpec.Spec.Containers[0].Command, nil
}
