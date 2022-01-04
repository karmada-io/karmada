package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/klog/v2"
)

// serviceLabels remove via Labels karmada service
var serviceLabels = map[string]string{"karmada.io/bootstrapping": "service-defaults"}

// CreateService create service
func (i *CommandInitOption) CreateService(service *corev1.Service) error {
	serviceClient := i.KubeClientSet.CoreV1().Services(i.Namespace)

	serviceList, err := serviceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Update if service exists.
	for _, v := range serviceList.Items {
		if service.Name == v.Name {
			t := &applycorev1.ServiceApplyConfiguration{
				TypeMetaApplyConfiguration: applymetav1.TypeMetaApplyConfiguration{
					APIVersion: &service.APIVersion,
					Kind:       &service.Kind,
				},
				ObjectMetaApplyConfiguration: &applymetav1.ObjectMetaApplyConfiguration{
					Name:      &service.Name,
					Namespace: &service.Namespace,
				},
				Spec: &applycorev1.ServiceSpecApplyConfiguration{
					ClusterIP:  &service.Spec.ClusterIP,
					ClusterIPs: service.Spec.ClusterIPs,
					Type:       &service.Spec.Type,
					Selector:   service.Spec.Selector,
				},
			}

			_, err = serviceClient.Apply(context.TODO(), t, metav1.ApplyOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
				},
				FieldManager: "apply",
			})
			if err != nil {
				return fmt.Errorf("apply service %s failed: %v", service.Name, err)
			}
			klog.Infof("service %s update successfully.", service.Name)
			return nil
		}
	}

	_, err = serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create service %s failed: %v", service.Name, err)
	}
	klog.Infof("service %s create successfully.", service.Name)
	return nil
}

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

func (i CommandInitOption) isNodePortExist() bool {
	svc, err := i.KubeClientSet.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Exit(err)
	}
	for _, v := range svc.Items {
		if v.Spec.Type != corev1.ServiceTypeNodePort {
			continue
		}
		if !nodePort(i.KarmadaAPIServerNodePort, v) {
			return false
		}
	}
	return true
}

func nodePort(nodePort int32, service corev1.Service) bool {
	for _, v := range service.Spec.Ports {
		if v.NodePort == nodePort {
			return false
		}
	}
	return true
}
