package util

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

// todo: this can get by kubectl api-resources

// ResourceKindMap get the Resource of a given kind
var ResourceKindMap = map[string]string{
	"ConfigMap":             "configmaps",
	"Namespace":             "namespaces",
	"PersistentVolumeClaim": "persistentvolumeclaims",
	"PersistentVolume":      "persistentvolumes",
	"Pod":                   "pods",
	"Secret":                "secrets",
	"Service":               "services",
	"Deployment":            "deployments",
	"DaemonSet":             "daemonsets",
	"StatefulSet":           "statefulsets",
	"ReplicaSet":            "replicasets",
	"CronJob":               "cronjobs",
	"Job":                   "jobs",
	"Ingress":               "ingresses",
}

// GetResourceStructure get resource yaml from kubernetes
func GetResourceStructure(client dynamic.Interface, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("can't get parse groupVersion[namespace: %s name: %s kind: %s]. error: %v", namespace,
			name, ResourceKindMap[kind], err)
	}
	dynamicResource := schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: ResourceKindMap[kind]}
	result, err := client.Resource(dynamicResource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get resource[namespace: %s name: %s kind: %s]. error: %v", namespace,
			name, ResourceKindMap[kind], err)
	}
	return result, nil
}

// GetMatchItems get match item by compare include items and exclude items
func GetMatchItems(includeItems, excludeItems []string) []string {
	includeSet := sets.NewString()
	excludeSet := sets.NewString()
	for _, targetItem := range excludeItems {
		excludeSet.Insert(targetItem)
	}
	for _, targetItem := range includeItems {
		includeSet.Insert(targetItem)
	}

	matchItems := includeSet.Difference(excludeSet)

	return matchItems.List()
}
