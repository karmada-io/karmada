package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsNamespaceExist tells if specific already exists.
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

// CreateNamespaceIfNotExist try to create the namespace if it does not exist.
func CreateNamespaceIfNotExist(client client.Client, namespaceObj *corev1.Namespace) error {
	namespace := &corev1.Namespace{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: namespaceObj.Name}, namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := client.Create(context.TODO(), namespaceObj); err != nil {
			return err
		}
	}
	return nil
}
