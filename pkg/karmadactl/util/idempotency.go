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
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// CreateService creates a Service if the target resource doesn't exist.
// If the resource exists already, return directly
func CreateService(client kubernetes.Interface, service *corev1.Service) error {
	if _, err := client.CoreV1().Services(service.Namespace).Create(context.TODO(), service, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Service: %v", err)
		}

		klog.Warningf("Service %s is existed, creation process will skip", service.Name)
	}
	return nil
}

// CreateOrUpdateSecret creates a Secret if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateSecret(client kubernetes.Interface, secret *corev1.Secret) error {
	if _, err := client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Secret: %v", err)
		}

		existSecret, err := client.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		secret.ResourceVersion = existSecret.ResourceVersion

		if _, err := client.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Secret: %v", err)
		}
	}
	klog.V(2).Infof("Secret %s/%s has been created or updated.", secret.Namespace, secret.Name)

	return nil
}

// CreateOrUpdateDeployment creates a Deployment if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateDeployment(client kubernetes.Interface, deploy *appsv1.Deployment) error {
	if _, err := client.AppsV1().Deployments(deploy.Namespace).Create(context.TODO(), deploy, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create Deployment: %v", err)
		}

		existDeployment, err := client.AppsV1().Deployments(deploy.Namespace).Get(context.TODO(), deploy.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		deploy.ResourceVersion = existDeployment.ResourceVersion

		if _, err := client.AppsV1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Deployment: %v", err)
		}
	}
	klog.V(2).Infof("Deployment %s/%s has been created or updated.", deploy.Namespace, deploy.Name)

	return nil
}

// CreateOrUpdateAPIService creates a ApiService if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateAPIService(apiRegistrationClient *aggregator.Clientset, apiservice *apiregistrationv1.APIService) error {
	if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Create(context.TODO(), apiservice, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create APIService: %v", err)
		}

		existAPIService, err := apiRegistrationClient.ApiregistrationV1().APIServices().Get(context.TODO(), apiservice.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		apiservice.ResourceVersion = existAPIService.ResourceVersion

		if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Update(context.TODO(), apiservice, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update APIService: %v", err)
		}
	}

	klog.V(2).Infof("APIService %s has been created or updated.", apiservice.Name)
	return nil
}

// CreateOrUpdateRole creates a Role if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateRole(client kubernetes.Interface, role *rbacv1.Role) error {
	if _, err := client.RbacV1().Roles(role.Namespace).Create(context.TODO(), role, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC role: %v", err)
		}

		existRole, err := client.RbacV1().Roles(role.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		role.ResourceVersion = existRole.ResourceVersion

		if _, err := client.RbacV1().Roles(role.Namespace).Update(context.TODO(), role, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC role: %v", err)
		}
	}
	klog.V(2).Infof("Role %s/%s has been created or updated.", role.Namespace, role.Name)

	return nil
}

// CreateOrUpdateRoleBinding creates a RoleBinding if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateRoleBinding(client kubernetes.Interface, roleBinding *rbacv1.RoleBinding) error {
	if _, err := client.RbacV1().RoleBindings(roleBinding.Namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC rolebinding: %v", err)
		}

		existRoleBinding, err := client.RbacV1().RoleBindings(roleBinding.Namespace).Get(context.TODO(), roleBinding.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		roleBinding.ResourceVersion = existRoleBinding.ResourceVersion

		if _, err := client.RbacV1().RoleBindings(roleBinding.Namespace).Update(context.TODO(), roleBinding, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC rolebinding: %v", err)
		}
	}
	klog.V(2).Infof("RoleBinding %s/%s has been created or updated.", roleBinding.Namespace, roleBinding.Name)

	return nil
}

// CreateOrUpdateClusterRole creates a ClusterRole if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateClusterRole(client kubernetes.Interface, clusterRole *rbacv1.ClusterRole) error {
	if _, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create ClusterRole: %v", err)
		}

		existClusterRole, err := client.RbacV1().ClusterRoles().Get(context.TODO(), clusterRole.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterRole.ResourceVersion = existClusterRole.ResourceVersion

		if _, err := client.RbacV1().ClusterRoles().Update(context.TODO(), clusterRole, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update ClusterRole: %v", err)
		}
	}
	klog.V(2).Infof("ClusterRole %s has been created or updated.", clusterRole.Name)

	return nil
}

// CreateOrUpdateClusterRoleBinding creates a ClusterRoleBinding if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateClusterRoleBinding(client kubernetes.Interface, clusterRoleBinding *rbacv1.ClusterRoleBinding) error {
	if _, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create ClusterRoleBinding: %v", err)
		}

		existCrb, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterRoleBinding.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterRoleBinding.ResourceVersion = existCrb.ResourceVersion

		if _, err := client.RbacV1().ClusterRoleBindings().Update(context.TODO(), clusterRoleBinding, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update ClusterRolebinding: %v", err)
		}
	}
	klog.V(2).Infof("ClusterRolebinding %s has been created or updated.", clusterRoleBinding.Name)

	return nil
}

// CreateOrUpdateConfigMap creates a ConfigMap if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateConfigMap(client *kubernetes.Clientset, cm *corev1.ConfigMap) error {
	if _, err := client.CoreV1().ConfigMaps(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create ConfigMap: %v", err)
		}

		existCm, err := client.CoreV1().ConfigMaps(cm.Namespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		cm.ResourceVersion = existCm.ResourceVersion

		if _, err := client.CoreV1().ConfigMaps(cm.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update ConfigMap: %v", err)
		}
	}
	klog.V(2).Infof("ConfigMap %s/%s has been created or updated.", cm.Namespace, cm.Name)

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

// CreateOrUpdateNamespace creates a Namespaces if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
func CreateOrUpdateNamespace(client kubernetes.Interface, ns *corev1.Namespace) error {
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
	klog.Infof("Namespace %s has been created or updated.", ns.Name)

	return nil
}

// CreateOrUpdateService creates a Service if the target resource doesn't exist.
// If the resource exists already, this function will update the resource instead.
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

		if _, err := client.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update Service: %v", err)
		}
	}
	klog.Infof("Service %s/%s has been created or updated.", svc.Namespace, svc.Name)

	return nil
}
