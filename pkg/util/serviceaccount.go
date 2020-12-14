package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

// CreateServiceAccount just try to create the ServiceAccount.
func CreateServiceAccount(client kubeclient.Interface, saObj *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	_, err := client.CoreV1().ServiceAccounts(saObj.Namespace).Create(context.TODO(), saObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return saObj, nil
		}

		return nil, err
	}

	return saObj, nil
}

// IsServiceAccountExist tells if specific service account already exists.
func IsServiceAccountExist(client kubeclient.Interface, namespace string, name string) (bool, error) {
	_, err := client.CoreV1().ServiceAccounts(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// DeleteServiceAccount just try to delete the ServiceAccount.
func DeleteServiceAccount(client kubeclient.Interface, namespace, name string) error {
	err := client.CoreV1().ServiceAccounts(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
