package util

import "k8s.io/apimachinery/pkg/runtime/schema"

func GetStsGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}
}
