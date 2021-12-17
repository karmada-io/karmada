package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

//ServiceAccountFromSpec sa spec
func (i *InstallOptions) ServiceAccountFromSpec(name []string) []corev1.ServiceAccount {
	var sa []corev1.ServiceAccount

	for _, v := range name {
		sa = append(sa, corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "V1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      v,
				Namespace: i.Namespace,
			},
		})
	}

	return sa
}

//CreateServiceAccount receive ServiceAccountFromSpec create sa
func (i *InstallOptions) CreateServiceAccount() error {
	serviceAccount := i.ServiceAccountFromSpec([]string{controllerManagerDeploymentAndServiceName, schedulerDeploymentNameAndServiceAccountName, webhookDeploymentAndServiceAccountAndServiceName})
	saClient := i.KubeClientSet.CoreV1().ServiceAccounts(i.Namespace)

	for v := range serviceAccount {
		if _, err := saClient.Get(context.TODO(), serviceAccount[v].Name, metav1.GetOptions{}); err == nil {
			klog.Warningf("ServiceAccount %s already exists. ", serviceAccount[v].Name)
			continue
		}
		if _, err := saClient.Create(context.TODO(), &serviceAccount[v], metav1.CreateOptions{}); err != nil {
			klog.Errorf("Create ServiceAccount %s failed: %v", serviceAccount[v].Name)
			return err
		}
	}

	return nil
}
