package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/klog/v2"
)

// SecretFromSpec secret spec
func (i *CommandInitOption) SecretFromSpec(name string, secretType corev1.SecretType, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
			Labels:    map[string]string{"karmada.io/bootstrapping": "secret-defaults"},
		},
		//Immutable:  immutable,
		Type:       secretType,
		StringData: data,
	}
}

// CreateSecret receive SecretFromSpec create secret
func (i *CommandInitOption) CreateSecret(secret *corev1.Secret) error {
	if _, err := i.KubeClientSet.CoreV1().Secrets(i.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			t := &applycorev1.SecretApplyConfiguration{
				TypeMetaApplyConfiguration: applymetav1.TypeMetaApplyConfiguration{
					APIVersion: &secret.APIVersion,
					Kind:       &secret.Kind,
				},
				ObjectMetaApplyConfiguration: &applymetav1.ObjectMetaApplyConfiguration{
					Name:      &secret.Name,
					Namespace: &secret.Namespace,
				},
				Data:       secret.Data,
				StringData: secret.StringData,
				Type:       &secret.Type,
			}

			_, err = i.KubeClientSet.CoreV1().Secrets(i.Namespace).Apply(context.TODO(), t, metav1.ApplyOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: secret.APIVersion,
					Kind:       secret.Kind,
				},
				FieldManager: "apply",
			})
			if err != nil {
				return fmt.Errorf("secret '%s' Apply failed: %v", secret.Name, err)
			}
			klog.Infof("secret '%s' Apply successfully.", secret.Name)
			return nil
		}
		return err
	}

	klog.Infof("secret '%s' Create successfully.", secret.Name)
	return nil
}
