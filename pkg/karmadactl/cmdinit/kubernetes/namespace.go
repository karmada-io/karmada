package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// CreateNamespace namespace IfNotExist
func (i *CommandInitOption) CreateNamespace() error {
	_, err := i.KubeClientSet.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: i.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Infof("Namespace %s already exists.", i.Namespace)
			return nil
		}
		return err
	}
	klog.Infof("Create Namespace '%s' successfully.", i.Namespace)
	return nil
}
