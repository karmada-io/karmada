package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

// IsNamespaceExist tells if the namespace already exists.
func IsNamespaceExist(client kubeclient.Interface, namespace string) (bool, error) {
	_, err := client.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CreateNamespace just try to create the namespace.
func CreateNamespace(client kubeclient.Interface, namespaceObj *corev1.Namespace) (*corev1.Namespace, error) {
	_, err := client.CoreV1().Namespaces().Create(context.TODO(), namespaceObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return namespaceObj, nil
		}

		return nil, err
	}

	return namespaceObj, nil
}

// DeleteNamespace just try to delete the namespace.
func DeleteNamespace(client kubeclient.Interface, namespace string) error {
	err := client.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// EnsureNamespaceExist makes sure that the specific namespace exist in cluster.
// If namespace not exit, just create it.
func EnsureNamespaceExist(client kubeclient.Interface, namespace string, dryRun bool) (*corev1.Namespace, error) {
	namespaceObj := &corev1.Namespace{}
	namespaceObj.Name = namespace

	if dryRun {
		return namespaceObj, nil
	}

	// It's necessary to check if a namespace exists before creating it.
	// Sometimes the namespace is created in advance, to give less privilege to Karmada.
	exist, err := IsNamespaceExist(client, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check if namespace exist. namespace: %s, error: %v", namespace, err)
	}
	if exist {
		return namespaceObj, nil
	}

	createdObj, err := CreateNamespace(client, namespaceObj)
	if err != nil {
		return nil, fmt.Errorf("ensure namespace failed due to create failed. namespace: %s, error: %v", namespace, err)
	}

	return createdObj, nil
}
