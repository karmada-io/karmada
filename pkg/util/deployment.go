package util

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

// IsDeploymentExist tells if specific already exists.
func IsDeploymentExist(client kubeclient.Interface, namespace, name string) (bool, error) {
	_, err := client.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CreateDeployment just try to create the Deployment.
func CreateDeployment(client kubeclient.Interface, deploymentObj *appsv1.Deployment) (*appsv1.Deployment, error) {
	_, err := client.AppsV1().Deployments(deploymentObj.Namespace).Create(context.TODO(), deploymentObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return deploymentObj, nil
		}

		return nil, err
	}

	return deploymentObj, nil
}

// DeleteDeployment just try to delete the Deployment.
func DeleteDeployment(client kubeclient.Interface, namespace, name string) error {
	err := client.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
