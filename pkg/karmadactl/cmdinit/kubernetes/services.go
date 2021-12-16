package kubernetes

import (
	"context"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
)

// CreateService create service
func (i *InstallOptions) CreateService(service *corev1.Service) error {
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
				return errors.Errorf("Apply service %s failed: %v\n", service.Name, err)
			}
			klog.Infof("service %s update successfully.", service.Name)
			return nil
		}
	}

	_, err = serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return errors.Errorf("Create service %s failed: %v\n", service.Name, err)
	}
	klog.Infof("service %s create successfully.", service.Name)
	return nil
}

// makeEtcdService etcd service
func (i *InstallOptions) makeEtcdService(name string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
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

func (i *InstallOptions) makeKarmadaAPIServerService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: apiServerLabels,
			Ports: []corev1.ServicePort{
				{
					Name:     portName,
					Protocol: corev1.ProtocolTCP,
					Port:     options.KarmadaMasterPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: options.KarmadaMasterPort,
					},
				},
			},
		},
	}
}

func (i *InstallOptions) kubeControllerManagerService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
			Namespace: i.Namespace,
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

func (i *InstallOptions) karmadaWebhookService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookDeploymentAndServiceAccountAndServiceName,
			Namespace: i.Namespace,
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
