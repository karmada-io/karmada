/*
Copyright 2021 The Karmada Authors.

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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// replicaInterpreter is the function that used to parse replica and requirements from object.
type replicaInterpreter func(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error)

func getAllDefaultReplicaInterpreter() map[schema.GroupVersionKind]replicaInterpreter {
	s := make(map[schema.GroupVersionKind]replicaInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployReplica
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = statefulSetReplica
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobReplica
	s[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = podReplica
	return s
}

func deployReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	deploy := &appsv1.Deployment{}
	if err := helper.ConvertToTypedObject(object, deploy); err != nil {
		klog.Errorf("Failed to convert object(%s), err %v", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	var replica int32
	if deploy.Spec.Replicas != nil {
		replica = *deploy.Spec.Replicas
	}
	requirement := helper.GenerateReplicaRequirements(&deploy.Spec.Template)

	return replica, requirement, nil
}

func statefulSetReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	sts := &appsv1.StatefulSet{}
	if err := helper.ConvertToTypedObject(object, sts); err != nil {
		klog.Errorf("Failed to convert object(%s), err %v", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	var replica int32
	if sts.Spec.Replicas != nil {
		replica = *sts.Spec.Replicas
	}
	requirement := helper.GenerateReplicaRequirements(&sts.Spec.Template)

	return replica, requirement, nil
}

func jobReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	job := &batchv1.Job{}
	err := helper.ConvertToTypedObject(object, job)
	if err != nil {
		klog.Errorf("Failed to convert object(%s), err %v", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	var replica int32
	// parallelism might never be nil as the kube-apiserver will set it to 1 by default if not specified.
	if job.Spec.Parallelism != nil {
		replica = *job.Spec.Parallelism
	}
	// For fixed completion count Jobs, the actual number of pods running in parallel will not exceed the number of remaining completions.
	// Higher values of .spec.parallelism are effectively ignored.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/job/
	completions := ptr.Deref[int32](job.Spec.Completions, replica)
	if replica > completions {
		replica = completions
	}
	requirement := helper.GenerateReplicaRequirements(&job.Spec.Template)

	return replica, requirement, nil
}

func podReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	pod := &corev1.Pod{}
	err := helper.ConvertToTypedObject(object, pod)
	if err != nil {
		klog.Errorf("Failed to convert object(%s), err %v", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	// For Pod, its replica is always 1.
	return 1, helper.GenerateReplicaRequirements(&corev1.PodTemplateSpec{Spec: pod.Spec}), nil
}
