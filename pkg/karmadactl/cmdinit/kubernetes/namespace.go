package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// CreateNamespace namespace IfNotExist
func (i *CommandInitOption) CreateNamespace() error {
	namespaceClient := i.KubeClientSet.CoreV1().Namespaces()
	namespaceList, err := namespaceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, nsList := range namespaceList.Items {
		if i.Namespace == nsList.Name {
			klog.Infof("Namespace %s already exists.", i.Namespace)
			return nil
		}
	}

	n := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: i.Namespace,
		},
	}

	_, err = namespaceClient.Create(context.TODO(), n, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	klog.Infof("Create Namespace '%s' successfully.", i.Namespace)
	return nil
}
