package defaultinterpreter

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type healthInterpreter func(object *unstructured.Unstructured) (bool, error)

func getAllDefaultHealthInterpreter() map[schema.GroupVersionKind]healthInterpreter {
	s := make(map[schema.GroupVersionKind]healthInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = interpretDeploymentHealth
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = interpretStatefulSetHealth
	return s
}

func interpretDeploymentHealth(object *unstructured.Unstructured) (bool, error) {
	deploy := &appsv1.Deployment{}
	err := helper.ConvertToTypedObject(object, deploy)
	if err != nil {
		return false, err
	}

	healthy := (deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas) && (deploy.Generation == deploy.Status.ObservedGeneration)
	return healthy, nil
}

func interpretStatefulSetHealth(object *unstructured.Unstructured) (bool, error) {
	statefulSet := &appsv1.StatefulSet{}
	err := helper.ConvertToTypedObject(object, &statefulSet)
	if err != nil {
		return false, err
	}

	healthy := (statefulSet.Status.UpdatedReplicas == *statefulSet.Spec.Replicas) && (statefulSet.Generation == statefulSet.Status.ObservedGeneration)
	return healthy, nil
}
