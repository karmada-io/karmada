package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

//
func generateGroupVersionResource(apiVersion, kind string) (schema.GroupVersionResource, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	dynamicResource := schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: ResourceKindMap[kind]}
	return dynamicResource, nil
}

// GetResourceStructure get resource yaml from kubernetes
func GetResourceStructure(client dynamic.Interface, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
	dynamicResource, err := generateGroupVersionResource(apiVersion, kind)
	if err != nil {
		return nil, err
	}
	result, err := client.Resource(dynamicResource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetResourcesStructureByFilter get resources yaml from kubernetes by filter
func GetResourcesStructureByFilter(client dynamic.Interface, apiVersion, kind, namespace string, labelSelector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	dynamicResource, err := generateGroupVersionResource(apiVersion, kind)
	if err != nil {
		return nil, err
	}
	result, err := client.Resource(dynamicResource).Namespace(namespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetMatchItems get match item by compare include items and exclude items
func GetMatchItems(includeItems, excludeItems []string) []string {
	if includeItems == nil {
		includeItems = []string{}
	}
	if excludeItems == nil {
		excludeItems = []string{}
	}
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

// GetDeduplicationArray get deduplication array
func GetDeduplicationArray(list []string) []string {
	if list == nil {
		return []string{}
	}
	result := sets.String{}
	for _, item := range list {
		result.Insert(item)
	}
	return result.List()
}
