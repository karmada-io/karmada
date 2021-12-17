package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/klog/v2"
)

//SecretFromSpec secret spec
func (i *InstallOptions) SecretFromSpec(name string, secretType corev1.SecretType, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
		},
		//Immutable:  immutable,
		Type:       secretType,
		StringData: data,
	}
}

//CreateSecret receive SecretFromSpec create secret
func (i *InstallOptions) CreateSecret(secret *corev1.Secret) error {
	secretClient := i.KubeClientSet.CoreV1().Secrets(i.Namespace)

	secretList, err := secretClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Update if secret exists.
	for _, v := range secretList.Items {
		if secret.Name == v.Name {
			t := &applycorev1.SecretApplyConfiguration{
				TypeMetaApplyConfiguration: applymetav1.TypeMetaApplyConfiguration{
					APIVersion: &secret.APIVersion,
					Kind:       &secret.Kind,
				},
				ObjectMetaApplyConfiguration: &applymetav1.ObjectMetaApplyConfiguration{
					Name:      &secret.Name,
					Namespace: &secret.Namespace,
				},
				Immutable:  v.Immutable,
				Data:       secret.Data,
				StringData: secret.StringData,
				Type:       &secret.Type,
			}

			_, err = secretClient.Apply(context.TODO(), t, metav1.ApplyOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: secret.APIVersion,
					Kind:       secret.Kind,
				},
				FieldManager: "apply",
			})
			if err != nil {
				return fmt.Errorf("apply secret %s failed: %v", secret.Name, err)
			}
			klog.Infof("secret %s update successfully.", secret.Name)
			return nil
		}
	}

	_, err = secretClient.Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create secret %s failed: %v", secret.Name, err)
	}
	klog.Infof("secret %s Create successfully.", secret.Name)
	return nil
}
