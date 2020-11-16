package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetTargetSecret will get secrets(type=targetType, namespace=targetNamespace) from a list of secret references.
func GetTargetSecret(client kubeclient.Interface, secretReferences []corev1.ObjectReference, targetType corev1.SecretType, targetNamespace string) (*corev1.Secret, error) {
	for _, objectReference := range secretReferences {
		klog.V(2).Infof("checking secret: %s/%s", targetNamespace, objectReference.Name)
		secret, err := client.CoreV1().Secrets(targetNamespace).Get(context.TODO(), objectReference.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if secret.Type == targetType {
			return secret, nil
		}
	}

	return nil, fmt.Errorf("no specific secret found in namespace: %s", targetNamespace)
}

// CreateSecret just try to create the secret.
func CreateSecret(client kubeclient.Interface, secret *corev1.Secret) (*corev1.Secret, error) {
	st, err := client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return st, nil
		}

		return nil, err
	}

	return st, nil
}
