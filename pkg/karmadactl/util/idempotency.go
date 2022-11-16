package util

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// CreateService creates a Service if the target resource doesn't exist. If the resource exists already, return directly
func CreateService(client kubeclient.Interface, service *corev1.Service) error {
	if _, err := client.CoreV1().Services(service.ObjectMeta.Namespace).Create(context.TODO(), service, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Service: %v", err)
		}

		klog.Warningf("Service %s is existed, creation process will skip", service.ObjectMeta.Name)
	}
	return nil
}

// CreateOrUpdateSecret creates a Secret if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateSecret(client kubeclient.Interface, secret *corev1.Secret) error {
	if _, err := client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Secret: %v", err)
		}

		existSecret, err := client.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		secret.ResourceVersion = existSecret.ResourceVersion

		if _, err := client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Secret: %v", err)
		}
	}
	return nil
}

// CreateOrUpdateDeployment creates a Deployment if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateDeployment(client kubeclient.Interface, deploy *appsv1.Deployment) error {
	if _, err := client.AppsV1().Deployments(deploy.Namespace).Create(context.TODO(), deploy, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Deployment: %v", err)
		}

		existDeployment, err := client.AppsV1().Deployments(deploy.Namespace).Get(context.TODO(), deploy.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		deploy.ResourceVersion = existDeployment.ResourceVersion

		if _, err := client.AppsV1().Deployments(deploy.ObjectMeta.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Deployment: %v", err)
		}
	}
	return nil
}

// CreateOrUpdateAPIService creates a ApiService if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateAPIService(apiRegistrationClient *aggregator.Clientset, apiservice *apiregistrationv1.APIService) error {
	if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Create(context.TODO(), apiservice, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create APIService: %v", err)
		}

		existAPIService, err := apiRegistrationClient.ApiregistrationV1().APIServices().Get(context.TODO(), apiservice.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		apiservice.ObjectMeta.ResourceVersion = existAPIService.ObjectMeta.ResourceVersion

		if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Update(context.TODO(), apiservice, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update APIService: %v", err)
		}
	}
	return nil
}

// CreateOrUpdateRole creates a Role if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateRole(client kubeclient.Interface, role *rbacv1.Role) error {
	if _, err := client.RbacV1().Roles(role.ObjectMeta.Namespace).Create(context.TODO(), role, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC role: %v", err)
		}

		existRole, err := client.RbacV1().Roles(role.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		role.ResourceVersion = existRole.ResourceVersion

		if _, err := client.RbacV1().Roles(role.ObjectMeta.Namespace).Update(context.TODO(), role, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC role: %v", err)
		}
	}
	klog.V(2).Infof("Role %s%s has been created or updated.", role.ObjectMeta.Namespace, role.ObjectMeta.Name)

	return nil
}

// CreateOrUpdateRoleBinding creates a RoleBinding if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateRoleBinding(client kubeclient.Interface, roleBinding *rbacv1.RoleBinding) error {
	if _, err := client.RbacV1().RoleBindings(roleBinding.ObjectMeta.Namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC rolebinding: %v", err)
		}

		existRoleBinding, err := client.RbacV1().RoleBindings(roleBinding.Namespace).Get(context.TODO(), roleBinding.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		roleBinding.ResourceVersion = existRoleBinding.ResourceVersion

		if _, err := client.RbacV1().RoleBindings(roleBinding.ObjectMeta.Namespace).Update(context.TODO(), roleBinding, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC rolebinding: %v", err)
		}
	}
	klog.V(2).Infof("RoleBinding %s/%s has been created or updated.", roleBinding.ObjectMeta.Namespace, roleBinding.ObjectMeta.Name)

	return nil
}

// CreateOrUpdateConfigMap creates a ConfigMap if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateConfigMap(client *kubeclient.Clientset, cm *corev1.ConfigMap) error {
	if _, err := client.CoreV1().ConfigMaps(cm.ObjectMeta.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create ConfigMap: %v", err)
		}

		existCm, err := client.CoreV1().ConfigMaps(cm.Namespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		cm.ResourceVersion = existCm.ResourceVersion

		if _, err := client.CoreV1().ConfigMaps(cm.ObjectMeta.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update ConfigMap: %v", err)
		}
	}
	klog.V(2).Infof("ConfigMap %s/%s has been created or updated.", cm.ObjectMeta.Namespace, cm.ObjectMeta.Name)

	return nil
}

// NewNamespace generates a new Namespace by given name.
func NewNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateOrUpdateNamespace creates a Namespaces if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateNamespace(client kubeclient.Interface, ns *corev1.Namespace) error {
	if _, err := client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Namespace: %v", err)
		}

		existNs, err := client.CoreV1().Namespaces().Get(context.TODO(), ns.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		ns.ResourceVersion = existNs.ResourceVersion

		if _, err := client.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Namespace: %v", err)
		}
	}
	klog.Infof("Namespace %s has been created or updated.", ns.ObjectMeta.Name)

	return nil
}

// CreateOrUpdateService creates a Service if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateService(client kubernetes.Interface, svc *corev1.Service) error {
	if _, err := client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Service: %v", err)
		}

		existSvc, err := client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		svc.ResourceVersion = existSvc.ResourceVersion

		if _, err := client.CoreV1().Services(svc.ObjectMeta.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Service: %v", err)
		}
	}
	klog.Infof("Service %s/%s has been created or updated.", svc.ObjectMeta.Namespace, svc.ObjectMeta.Name)

	return nil
}
