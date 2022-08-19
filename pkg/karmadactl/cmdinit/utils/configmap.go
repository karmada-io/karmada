package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CreateOrUpdateConfigMap creates a ConfigMap if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateConfigMap(clientSet *kubernetes.Clientset, cm *corev1.ConfigMap) error {
	if _, err := clientSet.CoreV1().ConfigMaps(cm.ObjectMeta.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create ConfigMap: %v", err)
		}

		if _, err := clientSet.CoreV1().ConfigMaps(cm.ObjectMeta.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update ConfigMap: %v", err)
		}
	}
	klog.Infof("ConfigMap %s/%s has been created or updated.", cm.ObjectMeta.Namespace, cm.ObjectMeta.Name)

	return nil
}
