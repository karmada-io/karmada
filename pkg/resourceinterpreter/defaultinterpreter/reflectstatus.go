package defaultinterpreter

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type reflectStatusInterpreter func(object *unstructured.Unstructured) (*runtime.RawExtension, error)

func getAllDefaultReflectStatusInterpreter() map[schema.GroupVersionKind]reflectStatusInterpreter {
	s := make(map[schema.GroupVersionKind]reflectStatusInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = reflectDeploymentStatus
	return s
}

func reflectDeploymentStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if !exist {
		klog.Errorf("Failed to grab status from %s(%s/%s) which should have status field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}

	deploymentStatus, err := helper.ConvertToDeploymentStatus(statusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DeploymentStatus from map[string]interface{}: %v", err)
	}

	grabStatus := appsv1.DeploymentStatus{
		Replicas:            deploymentStatus.Replicas,
		UpdatedReplicas:     deploymentStatus.UpdatedReplicas,
		ReadyReplicas:       deploymentStatus.ReadyReplicas,
		AvailableReplicas:   deploymentStatus.AvailableReplicas,
		UnavailableReplicas: deploymentStatus.UnavailableReplicas,
	}
	return helper.BuildStatusRawExtension(grabStatus)
}

func getWholeStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if exist {
		return helper.BuildStatusRawExtension(statusMap)
	}
	return nil, nil
}
