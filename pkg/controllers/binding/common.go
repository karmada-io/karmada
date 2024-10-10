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

package binding

import (
	"context"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// workSyncer is design for syncing work objects as expected.
type workSyncer struct {
	client      client.Client
	interpreter resourceinterpreter.ResourceInterpreter
	overrider   overridemanager.OverrideManager
}

// extraConfig encapsulates additional input configurations for work synchronization.
type extraConfig struct {
	jobCompletions []workv1alpha2.TargetCluster
}

// ensureWorks ensure Works to be created or updated in all target clusters.
func (s *workSyncer) ensureWorks(ctx context.Context, workload *unstructured.Unstructured, binding metav1.Object) error {
	var bindingSpec workv1alpha2.ResourceBindingSpec
	if len(binding.GetNamespace()) > 0 {
		bindingObj := binding.(*workv1alpha2.ResourceBinding)
		bindingSpec = bindingObj.Spec
	} else {
		bindingObj := binding.(*workv1alpha2.ClusterResourceBinding)
		bindingSpec = bindingObj.Spec
	}

	targetClusters := calcTargetClusters(bindingSpec.Clusters, bindingSpec.RequiredBy)

	jobCompletions, err := divideReplicasByJobCompletions(workload, targetClusters)
	if err != nil {
		return err
	}
	config := extraConfig{jobCompletions: jobCompletions}

	for _, targetCluster := range targetClusters {
		err = s.ensureWork(ctx, workload.DeepCopy(), binding, targetCluster, config)
		if err != nil {
			klog.Errorf("Failed to ensure work for workload(%s/%s/%s) in cluster %s, err is: %v",
				workload.GetKind(), workload.GetNamespace(), workload.GetName(), targetCluster.Name, err)
			return err
		}
	}
	return nil
}

// ensureWork ensure Work to be created or updated in the target cluster.
func (s *workSyncer) ensureWork(ctx context.Context, workload *unstructured.Unstructured, binding metav1.Object, targetCluster workv1alpha2.TargetCluster, config extraConfig) error {
	var bindingSpec workv1alpha2.ResourceBindingSpec
	if len(binding.GetNamespace()) > 0 {
		bindingObj := binding.(*workv1alpha2.ResourceBinding)
		bindingSpec = bindingObj.Spec
	} else {
		bindingObj := binding.(*workv1alpha2.ClusterResourceBinding)
		bindingSpec = bindingObj.Spec
	}

	workload, err := s.reviseWorkload(bindingSpec, workload, targetCluster, config)
	if err != nil {
		return err
	}

	// We should call ApplyOverridePolicies last, as override rules have the highest priority
	cops, ops, err := s.overrider.ApplyOverridePolicies(workload, targetCluster.Name)
	if err != nil {
		klog.Errorf("Failed to apply overrides for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	workLabels := setupWorkLabels(binding)
	workload = mergeLabels(workload, workLabels)

	workAnnotations := setupWorkAnnotations(binding)
	workload = mergeAnnotations(workload, workAnnotations)

	workAnnotations = setupConflictResolution(workload, bindingSpec.ConflictResolution, workAnnotations)
	workAnnotations, err = RecordAppliedOverrides(cops, ops, workAnnotations)
	if err != nil {
		klog.Errorf("Failed to record appliedOverrides, Error: %v", err)
		return err
	}

	workMeta := metav1.ObjectMeta{
		Name:        names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()),
		Namespace:   names.GenerateExecutionSpaceName(targetCluster.Name),
		Finalizers:  []string{util.ExecutionControllerFinalizer},
		Labels:      workLabels,
		Annotations: workAnnotations,
	}

	suspendDispatching := shouldSuspendDispatching(bindingSpec.Suspension, targetCluster.Name)

	return helper.CreateOrUpdateWork(ctx, s.client, workMeta, workload, &suspendDispatching)
}

func calcTargetClusters(targetClusters []workv1alpha2.TargetCluster, requiredByBindingSnapshot []workv1alpha2.BindingSnapshot) []workv1alpha2.TargetCluster {
	if len(requiredByBindingSnapshot) == 0 {
		return targetClusters
	}

	scheduledClusterNames := util.ConvertToClusterNames(targetClusters)

	for _, requiredByBinding := range requiredByBindingSnapshot {
		for _, targetCluster := range requiredByBinding.Clusters {
			if !scheduledClusterNames.Has(targetCluster.Name) {
				scheduledClusterNames.Insert(targetCluster.Name)
				targetClusters = append(targetClusters, targetCluster)
			}
		}
	}

	return targetClusters
}

func mergeLabels(workload *unstructured.Unstructured, labels map[string]string) *unstructured.Unstructured {
	for k, v := range labels {
		util.MergeLabel(workload, k, v)
	}
	return workload
}

func setupWorkLabels(binding metav1.Object) map[string]string {
	var labels = make(map[string]string)
	if len(binding.GetNamespace()) > 0 {
		bindingID := util.GetLabelValue(binding.GetLabels(), workv1alpha2.ResourceBindingPermanentIDLabel)
		labels[workv1alpha2.ResourceBindingPermanentIDLabel] = bindingID
	} else {
		bindingID := util.GetLabelValue(binding.GetLabels(), workv1alpha2.ClusterResourceBindingPermanentIDLabel)
		labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel] = bindingID
	}
	return labels
}

func mergeAnnotations(workload *unstructured.Unstructured, annotations map[string]string) *unstructured.Unstructured {
	if workload.GetGeneration() > 0 {
		util.MergeAnnotation(workload, workv1alpha2.ResourceTemplateGenerationAnnotationKey, strconv.FormatInt(workload.GetGeneration(), 10))
	}

	for k, v := range annotations {
		util.MergeAnnotation(workload, k, v)
	}
	return workload
}

func setupWorkAnnotations(binding metav1.Object) map[string]string {
	annotations := make(map[string]string)
	if len(binding.GetNamespace()) > 0 {
		annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey] = binding.GetNamespace()
		annotations[workv1alpha2.ResourceBindingNameAnnotationKey] = binding.GetName()
	} else {
		annotations[workv1alpha2.ClusterResourceBindingAnnotationKey] = binding.GetName()
	}
	return annotations
}

// RecordAppliedOverrides record applied (cluster) overrides to annotations
func RecordAppliedOverrides(cops *overridemanager.AppliedOverrides, ops *overridemanager.AppliedOverrides,
	annotations map[string]string) (map[string]string, error) {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if cops != nil {
		appliedBytes, err := cops.MarshalJSON()
		if err != nil {
			return nil, err
		}
		if appliedBytes != nil {
			annotations[util.AppliedClusterOverrides] = string(appliedBytes)
		}
	}

	if ops != nil {
		appliedBytes, err := ops.MarshalJSON()
		if err != nil {
			return nil, err
		}
		if appliedBytes != nil {
			annotations[util.AppliedOverrides] = string(appliedBytes)
		}
	}

	return annotations, nil
}

// setupConflictResolution determine the conflictResolution annotation of Work: preferentially inherit from RT, then RB
func setupConflictResolution(workload *unstructured.Unstructured, conflictResolutionInBinding policyv1alpha1.ConflictResolution,
	annotations map[string]string) map[string]string {
	// conflictResolutionInRT refer to the annotation in ResourceTemplate
	conflictResolutionInRT := util.GetAnnotationValue(workload.GetAnnotations(), workv1alpha2.ResourceConflictResolutionAnnotation)

	// the final conflictResolution annotation value of Work inherit from RT preferentially
	// so if conflictResolution annotation is defined in RT already, just copy the value and return
	if conflictResolutionInRT == workv1alpha2.ResourceConflictResolutionOverwrite || conflictResolutionInRT == workv1alpha2.ResourceConflictResolutionAbort {
		annotations[workv1alpha2.ResourceConflictResolutionAnnotation] = conflictResolutionInRT
		return annotations
	} else if conflictResolutionInRT != "" {
		// ignore its value and add logs if conflictResolutionInRT is neither abort nor overwrite.
		klog.Warningf("Ignore the invalid conflict-resolution annotation in ResourceTemplate %s/%s/%s: %s",
			workload.GetKind(), workload.GetNamespace(), workload.GetName(), conflictResolutionInRT)
	}

	if conflictResolutionInBinding == policyv1alpha1.ConflictOverwrite {
		annotations[workv1alpha2.ResourceConflictResolutionAnnotation] = workv1alpha2.ResourceConflictResolutionOverwrite
		return annotations
	}

	annotations[workv1alpha2.ResourceConflictResolutionAnnotation] = workv1alpha2.ResourceConflictResolutionAbort
	return annotations
}

func divideReplicasByJobCompletions(workload *unstructured.Unstructured, clusters []workv1alpha2.TargetCluster) ([]workv1alpha2.TargetCluster, error) {
	if workload.GetKind() != util.JobKind {
		return nil, nil
	}

	var targetClusters []workv1alpha2.TargetCluster
	completions, found, err := unstructured.NestedInt64(workload.Object, util.SpecField, util.CompletionsField)
	if err != nil {
		return nil, err
	}

	if found {
		targetClusters = helper.SpreadReplicasByTargetClusters(int32(completions), clusters, nil)
	}

	return targetClusters, nil
}

func (s *workSyncer) reviseWorkload(bindingSpec workv1alpha2.ResourceBindingSpec, workload *unstructured.Unstructured, targetCluster workv1alpha2.TargetCluster, config extraConfig) (*unstructured.Unstructured, error) {
	var err error
	// If and only if the resource template has replicas, and the replica scheduling policy is divided,
	// we need to revise replicas.
	if needReviseReplicas(bindingSpec.Replicas, bindingSpec.Placement) {
		if s.interpreter.HookEnabled(workload.GroupVersionKind(), configv1alpha1.InterpreterOperationReviseReplica) {
			workload, err = s.interpreter.ReviseReplica(workload, int64(targetCluster.Replicas))
			if err != nil {
				klog.Errorf("Failed to revise replica for workload(%s/%s/%s) in cluster %s, err is: %v",
					workload.GetKind(), workload.GetNamespace(), workload.GetName(), targetCluster.Name, err)
				return nil, err
			}
		}

		// Set allocated completions for Job only when the '.spec.completions' field not omitted from resource template.
		// For jobs running with a 'work queue' usually leaves '.spec.completions' unset, in that case we skip
		// setting this field as well.
		// Refer to: https://kubernetes.io/docs/concepts/workloads/controllers/job/#parallel-jobs.
		if len(config.jobCompletions) > 0 {
			var completionReplicas int32
			for _, completion := range config.jobCompletions {
				if completion.Name == targetCluster.Name {
					completionReplicas = completion.Replicas
				}
			}

			if err = helper.ApplyReplica(workload, int64(completionReplicas), util.CompletionsField); err != nil {
				klog.Errorf("Failed to apply Completions for workload(%s/%s/%s) in cluster %s, err is: %v",
					workload.GetKind(), workload.GetNamespace(), workload.GetName(), targetCluster.Name, err)
				return nil, err
			}
		}
	}
	return workload, nil
}

func needReviseReplicas(replicas int32, placement *policyv1alpha1.Placement) bool {
	return replicas > 0 && placement != nil && placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDivided
}

func shouldSuspendDispatching(suspension *policyv1alpha1.Suspension, targetCluster string) bool {
	if suspension == nil {
		return false
	}

	suspendDispatching := ptr.Deref(suspension.Dispatching, false)

	if !suspendDispatching && suspension.DispatchingOnClusters != nil {
		for _, cluster := range suspension.DispatchingOnClusters.ClusterNames {
			if cluster == targetCluster {
				suspendDispatching = true
				break
			}
		}
	}
	return suspendDispatching
}
