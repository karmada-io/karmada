package defaultinterpreter

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

type dependenciesInterpreter func(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error)

func getAllDefaultDependenciesInterpreter() map[schema.GroupVersionKind]dependenciesInterpreter {
	s := make(map[schema.GroupVersionKind]dependenciesInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = getDeploymentDependencies
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = getJobDependencies
	s[batchv1.SchemeGroupVersion.WithKind(util.CronJobKind)] = getCronJobDependencies
	s[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = getPodDependencies
	s[appsv1.SchemeGroupVersion.WithKind(util.DaemonSetKind)] = getDaemonSetDependencies
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = getStatefulSetDependencies
	return s
}

func getDeploymentDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	deploymentObj := &appsv1.Deployment{}
	if err := helper.ConvertToTypedObject(object, deploymentObj); err != nil {
		return nil, fmt.Errorf("failed to convert Deployment from unstructured object: %v", err)
	}

	podObj, err := lifted.GetPodFromTemplate(&deploymentObj.Spec.Template, deploymentObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getJobDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	jobObj := &batchv1.Job{}
	err := helper.ConvertToTypedObject(object, jobObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Job from unstructured object: %v", err)
	}

	podObj, err := lifted.GetPodFromTemplate(&jobObj.Spec.Template, jobObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getCronJobDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	cronjobObj := &batchv1.CronJob{}
	err := helper.ConvertToTypedObject(object, cronjobObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert CronJob from unstructured object: %v", err)
	}

	podObj, err := lifted.GetPodFromTemplate(&cronjobObj.Spec.JobTemplate.Spec.Template, cronjobObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getPodDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	podObj := &corev1.Pod{}
	err := helper.ConvertToTypedObject(object, podObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Pod from unstructured object: %v", err)
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getDaemonSetDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	daemonSetObj := &appsv1.DaemonSet{}
	err := helper.ConvertToTypedObject(object, daemonSetObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DaemonSet from unstructured object: %v", err)
	}

	podObj, err := lifted.GetPodFromTemplate(&daemonSetObj.Spec.Template, daemonSetObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getStatefulSetDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	statefulSetObj := &appsv1.StatefulSet{}
	err := helper.ConvertToTypedObject(object, statefulSetObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert StatefulSet from unstructured object: %v", err)
	}

	podObj, err := lifted.GetPodFromTemplate(&statefulSetObj.Spec.Template, statefulSetObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getDependenciesFromPodTemplate(podObj *corev1.Pod) ([]configv1alpha1.DependentObjectReference, error) {
	dependentConfigMaps := getConfigMapNames(podObj)
	dependentSecrets := getSecretNames(podObj)
	dependentSas := getServiceAccountNames(podObj)
	dependentPVCs := getPVCNames(podObj)
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
	for sa := range dependentSas {
		dependentObjectRefs = append(dependentObjectRefs, configv1alpha1.DependentObjectReference{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
			Namespace:  podObj.Namespace,
			Name:       sa,
		})
	}

	for pvc := range dependentPVCs {
		dependentObjectRefs = append(dependentObjectRefs, configv1alpha1.DependentObjectReference{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
			Namespace:  podObj.Namespace,
			Name:       pvc,
		})
	}

	return dependentObjectRefs, nil
}

func getSecretNames(pod *corev1.Pod) sets.Set[string] {
	result := sets.New[string]()
	lifted.VisitPodSecretNames(pod, func(name string) bool {
		result.Insert(name)
		return true
	})
	return result
}

func getServiceAccountNames(pod *corev1.Pod) sets.Set[string] {
	result := sets.New[string]()
	if pod.Spec.ServiceAccountName != "" && pod.Spec.ServiceAccountName != "default" {
		result.Insert(pod.Spec.ServiceAccountName)
	}
	return result
}

func getConfigMapNames(pod *corev1.Pod) sets.Set[string] {
	result := sets.New[string]()
	lifted.VisitPodConfigmapNames(pod, func(name string) bool {
		result.Insert(name)
		return true
	})
	return result
}

func getPVCNames(pod *corev1.Pod) sets.Set[string] {
	result := sets.New[string]()
	for i := range pod.Spec.Volumes {
		volume := pod.Spec.Volumes[i]
		if volume.PersistentVolumeClaim != nil {
			claimName := volume.PersistentVolumeClaim.ClaimName
			if len(claimName) != 0 {
				result.Insert(claimName)
			}
		}
	}
	return result
}
