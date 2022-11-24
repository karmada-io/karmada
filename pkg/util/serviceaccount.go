package util

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

// DeleteServiceAccount just try to delete the ServiceAccount.
func DeleteServiceAccount(client kubeclient.Interface, namespace, name string) error {
	err := client.CoreV1().ServiceAccounts(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// EnsureServiceAccountExist makes sure that the specific service account exist in cluster.
// If service account not exit, just create it.
func EnsureServiceAccountExist(client kubeclient.Interface, serviceAccountObj *corev1.ServiceAccount, dryRun bool) (*corev1.ServiceAccount, error) {
	if dryRun {
		return serviceAccountObj, nil
	}

	createdObj, err := CreateServiceAccount(client, serviceAccountObj)
	if err != nil {
		return nil, fmt.Errorf("ensure service account failed due to create failed. service account: %s/%s, error: %v", serviceAccountObj.Namespace, serviceAccountObj.Name, err)
	}

	return createdObj, nil
}

// WaitForServiceAccountSecretCreation wait the ServiceAccount's secret has been created.
func WaitForServiceAccountSecretCreation(client kubeclient.Interface, asObj *corev1.ServiceAccount) (*corev1.Secret, error) {
	var clusterSecret *corev1.Secret
	err := wait.Poll(1*time.Second, 30*time.Second, func() (done bool, err error) {
		serviceAccount, err := client.CoreV1().ServiceAccounts(asObj.Namespace).Get(context.TODO(), asObj.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to retrieve service account(%s/%s) from cluster, err: %v", asObj.Namespace, asObj.Name, err)
		}
		clusterSecret, err = GetSecret(client, serviceAccount.Namespace, serviceAccount.Name)
		if apierrors.IsNotFound(err) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: serviceAccount.Namespace,
					Name:      serviceAccount.Name,
					Annotations: map[string]string{
						corev1.ServiceAccountNameKey: serviceAccount.Name,
					},
				},
				Type: corev1.SecretTypeServiceAccountToken,
			}
			clusterSecret, err = CreateSecret(client, secret)
		}
		if err != nil {
			return false, err
		}
		if _, ok := clusterSecret.Data["token"]; !ok {
			return false, nil // wait for the token controller to populate the Data["token"] field
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get serviceAccount secret, error: %v", err)
	}
	return clusterSecret, nil
}
