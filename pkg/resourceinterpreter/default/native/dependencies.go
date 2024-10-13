/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package native

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/pkg/util/names"
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
	s[networkingv1.SchemeGroupVersion.WithKind(util.IngressKind)] = getIngressDependencies
	s[mcsv1alpha1.SchemeGroupVersion.WithKind(util.ServiceImportKind)] = getServiceImportDependencies
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

	return helper.GetDependenciesFromPodTemplate(podObj)
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

	return helper.GetDependenciesFromPodTemplate(podObj)
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

	return helper.GetDependenciesFromPodTemplate(podObj)
}

func getPodDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	podObj := &corev1.Pod{}
	err := helper.ConvertToTypedObject(object, podObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Pod from unstructured object: %v", err)
	}

	return helper.GetDependenciesFromPodTemplate(podObj)
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

	return helper.GetDependenciesFromPodTemplate(podObj)
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

	deps, err := helper.GetDependenciesFromPodTemplate(podObj)
	if err != nil {
		return nil, err
	}

	if len(statefulSetObj.Spec.VolumeClaimTemplates) == 0 {
		return deps, nil
	}

	// ignore the PersistentVolumeClaim dependency if it was created by the StatefulSet VolumeClaimTemplates
	// the PVC dependency is not needed because the StatefulSet will manage the pvcs in the member cluster,
	// if it exists here it was just a placeholder not a real PVC
	var validDeps []configv1alpha1.DependentObjectReference
	volumeClaimTemplateNames := sets.Set[string]{}
	for i := range statefulSetObj.Spec.VolumeClaimTemplates {
		volumeClaimTemplateNames.Insert(statefulSetObj.Spec.VolumeClaimTemplates[i].Name)
	}

	for i := range deps {
		if deps[i].Kind != util.PersistentVolumeClaimKind {
			validDeps = append(validDeps, deps[i])
			continue
		}
		if volumeClaimTemplateNames.Has(deps[i].Name) {
			continue
		}
		validDeps = append(validDeps, deps[i])
	}

	return validDeps, nil
}

func getIngressDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	ingressObj := &networkingv1.Ingress{}
	err := helper.ConvertToTypedObject(object, ingressObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Ingress from unstructured object: %v", err)
	}
	var dependentObjectRefs []configv1alpha1.DependentObjectReference
	for _, tls := range ingressObj.Spec.TLS {
		dependentObjectRefs = append(dependentObjectRefs, configv1alpha1.DependentObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  ingressObj.Namespace,
			Name:       tls.SecretName,
		})
	}
	return dependentObjectRefs, nil
}

func getServiceImportDependencies(object *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	svcImportObj := &mcsv1alpha1.ServiceImport{}
	err := helper.ConvertToTypedObject(object, svcImportObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ServiceImport from unstructured object: %v", err)
	}
	derivedServiceName := names.GenerateDerivedServiceName(svcImportObj.Name)
	return []configv1alpha1.DependentObjectReference{
		{
			APIVersion: "v1",
			Kind:       util.ServiceKind,
			Namespace:  svcImportObj.Namespace,
			Name:       derivedServiceName,
		},
		{
			APIVersion: "discovery.k8s.io/v1",
			Kind:       util.EndpointSliceKind,
			Namespace:  svcImportObj.Namespace,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					discoveryv1.LabelServiceName: derivedServiceName,
				},
			},
		},
	}, nil
}
