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
	deploymentObj, err := helper.ConvertToDeployment(object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Deployment from unstructured object: %v", err)
	}

	podObj, err := lifted.GetPodFromTemplate(&deploymentObj.Spec.Template, deploymentObj, nil)
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

	podObj, err := lifted.GetPodFromTemplate(&jobObj.Spec.Template, jobObj, nil)
	if err != nil {
		return nil, err
	}

	return getDependenciesFromPodTemplate(podObj)
}

func getCronJobDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	cronjobObj, err := helper.ConvertToCronJob(object)
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

	podObj, err := lifted.GetPodFromTemplate(&daemonSetObj.Spec.Template, daemonSetObj, nil)
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
	return dependentObjectRefs, nil
}

func getSecretNames(pod *corev1.Pod) sets.String {
	result := sets.NewString()
	lifted.VisitPodSecretNames(pod, func(name string) bool {
		result.Insert(name)
		return true
	})
	return result
}

func getServiceAccountNames(pod *corev1.Pod) sets.String {
	result := sets.NewString()
	if pod.Spec.ServiceAccountName != "" && pod.Spec.ServiceAccountName != "default" {
		result.Insert(pod.Spec.ServiceAccountName)
	}
	return result
}

func getConfigMapNames(pod *corev1.Pod) sets.String {
	result := sets.NewString()
	lifted.VisitPodConfigmapNames(pod, func(name string) bool {
		result.Insert(name)
		return true
	})
	return result
}
