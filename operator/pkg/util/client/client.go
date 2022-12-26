package client

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdateService creates or updates a service
func CreateOrUpdateService(c client.Client, svc *corev1.Service) error {
	got := &corev1.Service{}
	err := c.Get(context.TODO(), client.ObjectKeyFromObject(svc), got)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return c.Create(context.TODO(), svc)
	}
	svc.ResourceVersion = got.ResourceVersion
	return c.Update(context.TODO(), svc)
}

// CreateOrUpdateDeployment creates or updates a deployment
func CreateOrUpdateDeployment(c client.Client, deployment *appsv1.Deployment) error {
	got := &appsv1.Deployment{}
	err := c.Get(context.TODO(), client.ObjectKeyFromObject(deployment), got)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return c.Create(context.TODO(), deployment)
	}
	deployment.ResourceVersion = got.ResourceVersion
	return c.Update(context.TODO(), deployment)
}

// CreateOrUpdateStatefulSet creates or updates a statefulset
func CreateOrUpdateStatefulSet(c client.Client, statefulset *appsv1.StatefulSet) error {
	got := &appsv1.StatefulSet{}
	err := c.Get(context.TODO(), client.ObjectKeyFromObject(statefulset), got)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return c.Create(context.TODO(), statefulset)
	}
	statefulset.ResourceVersion = got.ResourceVersion
	return c.Update(context.TODO(), statefulset)
}

// CreateOrUpdateSecret creates or updates a secret
func CreateOrUpdateSecret(c client.Client, secret *corev1.Secret) error {
	got := &corev1.Secret{}
	err := c.Get(context.TODO(), client.ObjectKeyFromObject(secret), got)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return c.Create(context.TODO(), secret)
	}
	secret.ResourceVersion = got.ResourceVersion
	return c.Update(context.TODO(), secret)
}

// CreateOrUpdateConfigMap creates or updates a configmap
func CreateOrUpdateConfigMap(c client.Client, cm *corev1.ConfigMap) error {
	got := &corev1.ConfigMap{}
	err := c.Get(context.TODO(), client.ObjectKeyFromObject(cm), got)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return c.Create(context.TODO(), cm)
	}
	cm.ResourceVersion = got.ResourceVersion
	return c.Update(context.TODO(), cm)
}

// CreateOrUpdateAPIService creates or updates an apiservice
func CreateOrUpdateAPIService(c client.Client, apisvc *apiregistrationv1.APIService) error {
	got := &apiregistrationv1.APIService{}
	err := c.Get(context.TODO(), client.ObjectKeyFromObject(apisvc), got)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return c.Create(context.TODO(), apisvc)
	}
	apisvc.ResourceVersion = got.ResourceVersion
	return c.Update(context.TODO(), apisvc)
}
