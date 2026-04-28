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

package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
)

// serviceLabels remove via Labels karmada service
var serviceLabels = map[string]string{"karmada.io/bootstrapping": "service-defaults"}

// makeEtcdService etcd service
func (i *CommandInitOption) makeEtcdService(name string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
			Labels:    serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector:  etcdLabels,
			ClusterIP: "None",
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:     etcdContainerClientPortName,
					Protocol: corev1.ProtocolTCP,
					Port:     etcdContainerClientPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: etcdContainerClientPort,
					},
				},
				{
					Name:     etcdContainerServerPortName,
					Protocol: corev1.ProtocolTCP,
					Port:     etcdContainerServerPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: etcdContainerServerPort,
					},
				},
			},
		},
	}
}

func (i *CommandInitOption) makeKarmadaAPIServerService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: apiServerLabels,
			Ports: []corev1.ServicePort{
				{
					Name:     portName,
					Protocol: corev1.ProtocolTCP,
					Port:     karmadaAPIServerContainerPort,
					NodePort: i.KarmadaAPIServerNodePort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: karmadaAPIServerContainerPort,
					},
				},
			},
		},
	}
}

func (i *CommandInitOption) kubeControllerManagerService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: kubeControllerManagerLabels,
			Ports: []corev1.ServicePort{
				{
					Name:     portName,
					Protocol: corev1.ProtocolTCP,
					Port:     kubeControllerManagerPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: kubeControllerManagerPort,
					},
				},
			},
		},
	}
}

func (i *CommandInitOption) karmadaWebhookService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookDeploymentAndServiceAccountAndServiceName,
			Namespace: i.Namespace,
			Labels:    serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: webhookLabels,
			Ports: []corev1.ServicePort{
				{
					Name:     webhookPortName,
					Protocol: corev1.ProtocolTCP,
					Port:     webhookPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: webhookTargetPort,
					},
				},
			},
		},
	}
}

func (i *CommandInitOption) karmadaAggregatedAPIServerService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAggregatedAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    serviceLabels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: aggregatedAPIServerLabels,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 443,
					},
				},
			},
		},
	}
}

func (i *CommandInitOption) isNodePortExist() bool {
	svc, err := i.KubeClientSet.CoreV1().Services(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Exit(err)
	}
	for _, v := range svc.Items {
		if v.Spec.Type != corev1.ServiceTypeNodePort {
			continue
		}
		if nodePortExistsInSVC(i.KarmadaAPIServerNodePort, v) {
			return true
		}
	}
	return false
}

func nodePortExistsInSVC(nodePort int32, service corev1.Service) bool {
	for _, v := range service.Spec.Ports {
		if v.NodePort == nodePort {
			return true
		}
	}
	return false
}
