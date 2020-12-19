package helper

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewNamespace will build a Namespace object.
func NewNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: namespace,
		},
	}
}
