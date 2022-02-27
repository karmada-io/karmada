package defaultinterpreter

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type dependenciesInterpreter func(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error)

func getAllDefaultDependenciesInterpreter() map[schema.GroupVersionKind]dependenciesInterpreter {
	s := make(map[schema.GroupVersionKind]dependenciesInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = getDeploymentDependencies
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = getJobDependencies
	s[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = getPodDependencies
	s[appsv1.SchemeGroupVersion.WithKind(util.DaemonSetKind)] = getDaemonSetDependencies
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = getStatefulSetDependencies
	return s
}

func getDeploymentDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	deploymentObj, err := helper.ConvertToDeployment(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Deployment from unstructured object: %v", err)
	}

	podObj, err := GetPodFromTemplate(&deploymentObj.Spec.Template, deploymentObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getJobDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	jobObj, err := helper.ConvertToJob(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Job from unstructured object: %v", err)
	}

	podObj, err := GetPodFromTemplate(&jobObj.Spec.Template, jobObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getPodDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	podObj, err := helper.ConvertToPod(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Pod from unstructured object: %v", err)
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getDaemonSetDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	daemonSetObj, err := helper.ConvertToDaemonSet(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DaemonSet from unstructured object: %v", err)
	}

	podObj, err := GetPodFromTemplate(&daemonSetObj.Spec.Template, daemonSetObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getStatefulSetDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	statefulSetObj, err := helper.ConvertToStatefulSet(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert StatefulSet from unstructured object: %v", err)
	}

	podObj, err := GetPodFromTemplate(&statefulSetObj.Spec.Template, statefulSetObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getDependenciesFromPodTemplate(podObj *corev1.Pod) ([]configv1alpha1.DependentObjectReference, error) {
	dependentConfigMaps := getConfigMapNames(podObj)
	dependentSecrets := getSecretNames(podObj)

	var dependentObjectRefs []configv1alpha1.DependentObjectReference
	for cm := range dependentConfigMaps {
		dependentObjectRefs = append(dependentObjectRefs, configv1alpha1.DependentObjectReference{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  podObj.Namespace,
			Name:       cm,
		})
	}

	for secret := range dependentSecrets {
		dependentObjectRefs = append(dependentObjectRefs, configv1alpha1.DependentObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  podObj.Namespace,
			Name:       secret,
		})
	}

	return dependentObjectRefs, nil
}
