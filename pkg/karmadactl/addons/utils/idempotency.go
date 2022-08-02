package utils

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// CreateService creates a Service if the target resource doesn't exist. If the resource exists already, return directly
func CreateService(KubeClientSet *kubernetes.Clientset, service *corev1.Service) error {
	if _, err := KubeClientSet.CoreV1().Services(service.ObjectMeta.Namespace).Create(context.TODO(), service, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create service: %v", err)
		}

		klog.Warningf("Service %s is existed, creation process will skip", service.ObjectMeta.Name)
	}
	return nil
}

// CreateOrUpdateSecret creates a Secret if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateSecret(KubeClientSet *kubernetes.Clientset, secret *corev1.Secret) error {
	if _, err := KubeClientSet.CoreV1().Secrets(secret.ObjectMeta.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create service: %v", err)
		}

		if _, err := KubeClientSet.CoreV1().Secrets(secret.ObjectMeta.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update deployment: %v", err)
		}
	}
	return nil
}

// CreateOrUpdateDeployment creates a Deployment if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateDeployment(client kubernetes.Interface, deploy *appsv1.Deployment) error {
	if _, err := client.AppsV1().Deployments(deploy.ObjectMeta.Namespace).Create(context.TODO(), deploy, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create deployment: %v", err)
		}

		if _, err := client.AppsV1().Deployments(deploy.ObjectMeta.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update deployment: %v", err)
		}
	}
	return nil
}

// CreateOrUpdateAPIService creates a ApiService if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateAPIService(apiRegistrationClient *aggregator.Clientset, apiservice *apiregistrationv1.APIService) error {
	if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Create(context.TODO(), apiservice, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create apiService: %v", err)
		}

		existAPIService, err := apiRegistrationClient.ApiregistrationV1().APIServices().Get(context.TODO(), apiservice.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		apiservice.ObjectMeta.ResourceVersion = existAPIService.ObjectMeta.ResourceVersion

		if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Update(context.TODO(), apiservice, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update apiService: %v", err)
		}
	}
	return nil
}
