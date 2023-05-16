package helper

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewGroupRESTMapper make a fake RESTMapper in GroupVersion, by kind and scope.
func NewGroupRESTMapper(kind string, scope meta.RESTScope) meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	m.Add(corev1.SchemeGroupVersion.WithKind(kind), scope)
	return m
}
