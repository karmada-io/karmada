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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// ensureWork ensure Work to be created or updated.
func ensureWork(
	ctx context.Context, c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter, workload *unstructured.Unstructured,
	overrideManager overridemanager.OverrideManager, binding metav1.Object, scope apiextensionsv1.ResourceScope,
) error {
	bindingSpec := getBindingSpec(binding, scope)
	targetClusters := mergeTargetClusters(bindingSpec.Clusters, bindingSpec.RequiredBy)
	var err error
	var errs []error

	var jobCompletions []workv1alpha2.TargetCluster
	if workload.GetKind() == util.JobKind && needReviseJobCompletions(bindingSpec.Replicas, bindingSpec.Placement) {
		jobCompletions, err = divideReplicasByJobCompletions(workload, targetClusters)
		if err != nil {
			return err
		}
	}

	for i := range targetClusters {
		targetCluster := targetClusters[i]
		clonedWorkload := workload.DeepCopy()

		workNamespace := names.GenerateExecutionSpaceName(targetCluster.Name)

		// When syncing workloads to member clusters, the controller MUST strictly adhere to the scheduling results
		// specified in bindingSpec.Clusters for replica allocation, rather than using the replicas declared in the
		// workload's resource template.
		// This rule applies regardless of whether the workload distribution mode is "Divided" or "Duplicated".
		// Failing to do so could allow workloads to bypass the quota checks performed by the scheduler
		// (especially during scale-up operations) or skip queue validation when scheduling is suspended.
		if needReviseReplicas(bindingSpec.Replicas) {
			if resourceInterpreter.HookEnabled(clonedWorkload.GroupVersionKind(), configv1alpha1.InterpreterOperationReviseReplica) {
				clonedWorkload, err = resourceInterpreter.ReviseReplica(clonedWorkload, int64(targetCluster.Replicas))
				if err != nil {
					klog.ErrorS(err, "Failed to revise replica for workload in cluster.", "workloadKind", workload.GetKind(),
						"workloadNamespace", workload.GetNamespace(), "workloadName", workload.GetName(), "cluster", targetCluster.Name)
					errs = append(errs, err)
					continue
				}
			}
		}

		// jobSpec.Completions specifies the desired number of successfully finished pods the job should be run with.
		// When the replica scheduling policy is set to "divided", jobSpec.Completions should also be divided accordingly.
		// The weight assigned to each cluster roughly equals that cluster's jobSpec.Parallelism value. This approach helps
		// balance the execution time of the job across member clusters.
		if len(jobCompletions) > 0 {
			// Set allocated completions for Job only when the '.spec.completions' field not omitted from resource template.
			// For jobs running with a 'work queue' usually leaves '.spec.completions' unset, in that case we skip
			// setting this field as well.
			// Refer to: https://kubernetes.io/docs/concepts/workloads/controllers/job/#parallel-jobs.
			if err = helper.ApplyReplica(clonedWorkload, int64(jobCompletions[i].Replicas), util.CompletionsField); err != nil {
				klog.ErrorS(err, "Failed to apply Completions for workload in cluster.",
					"workloadKind", clonedWorkload.GetKind(), "workloadNamespace", clonedWorkload.GetNamespace(),
					"workloadName", clonedWorkload.GetName(), "cluster", targetCluster.Name)
				errs = append(errs, err)
				continue
			}
		}

		// We should call ApplyOverridePolicies last, as override rules have the highest priority
		cops, ops, err := overrideManager.ApplyOverridePolicies(clonedWorkload, targetCluster.Name)
		if err != nil {
			klog.ErrorS(err, "Failed to apply overrides for workload in cluster.",
				"workloadKind", clonedWorkload.GetKind(), "workloadNamespace", clonedWorkload.GetNamespace(),
				"workloadName", clonedWorkload.GetName(), "cluster", targetCluster.Name)
			errs = append(errs, err)
			continue
		}
		workLabel := mergeLabel(clonedWorkload, binding, scope)

		annotations := mergeAnnotations(clonedWorkload, binding, scope)
		annotations = mergeConflictResolution(clonedWorkload, bindingSpec.ConflictResolution, annotations)
		annotations, err = RecordAppliedOverrides(cops, ops, annotations)
		if err != nil {
			klog.ErrorS(err, "Failed to record appliedOverrides in cluster.", "cluster", targetCluster.Name)
			errs = append(errs, err)
			continue
		}

		if features.FeatureGate.Enabled(features.StatefulFailoverInjection) {
			// we need to figure out if the targetCluster is in the cluster we are going to migrate application to.
			// If yes, we have to inject the preserved label state to the clonedWorkload.
			clonedWorkload = injectReservedLabelState(bindingSpec, targetCluster, clonedWorkload, len(targetClusters))
		}

		workMeta := metav1.ObjectMeta{
			Name:        names.GenerateWorkName(clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace()),
			Namespace:   workNamespace,
			Finalizers:  []string{util.ExecutionControllerFinalizer},
			Labels:      workLabel,
			Annotations: annotations,
		}

		if err = ctrlutil.CreateOrUpdateWork(
			ctx,
			c,
			workMeta,
			clonedWorkload,
			ctrlutil.WithSuspendDispatching(shouldSuspendDispatching(bindingSpec.Suspension, targetCluster)),
			ctrlutil.WithPreserveResourcesOnDeletion(ptr.Deref(bindingSpec.PreserveResourcesOnDeletion, false)),
		); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return errors.NewAggregate(errs)
}

func getBindingSpec(binding metav1.Object, scope apiextensionsv1.ResourceScope) workv1alpha2.ResourceBindingSpec {
	var bindingSpec workv1alpha2.ResourceBindingSpec
	switch scope {
	case apiextensionsv1.NamespaceScoped:
		bindingObj := binding.(*workv1alpha2.ResourceBinding)
		bindingSpec = bindingObj.Spec
	case apiextensionsv1.ClusterScoped:
		bindingObj := binding.(*workv1alpha2.ClusterResourceBinding)
		bindingSpec = bindingObj.Spec
	}
	return bindingSpec
}

// injectReservedLabelState injects the reservedLabelState in to the failover to cluster.
// We have the following restrictions on whether to perform injection operations:
//  1. Only the scenario where an application is deployed in one cluster and migrated to
//     another cluster is considered.
//  2. If consecutive failovers occur, for example, an application is migrated form clusterA
//     to clusterB and then to clusterC, the PreservedLabelState before the last failover is
//     used for injection. If the PreservedLabelState is empty, the injection is skipped.
//  3. The injection operation is performed only when PurgeMode is set to Immediately.
func injectReservedLabelState(bindingSpec workv1alpha2.ResourceBindingSpec, moveToCluster workv1alpha2.TargetCluster, workload *unstructured.Unstructured, clustersLen int) *unstructured.Unstructured {
	if clustersLen > 1 {
		return workload
	}

	if len(bindingSpec.GracefulEvictionTasks) == 0 {
		return workload
	}
	targetEvictionTask := bindingSpec.GracefulEvictionTasks[len(bindingSpec.GracefulEvictionTasks)-1]

	if targetEvictionTask.PurgeMode != policyv1alpha1.Immediately {
		return workload
	}

	clustersBeforeFailover := sets.NewString(targetEvictionTask.ClustersBeforeFailover...)
	if clustersBeforeFailover.Has(moveToCluster.Name) {
		return workload
	}

	for key, value := range targetEvictionTask.PreservedLabelState {
		util.MergeLabel(workload, key, value)
	}

	return workload
}

func mergeTargetClusters(targetClusters []workv1alpha2.TargetCluster, requiredByBindingSnapshot []workv1alpha2.BindingSnapshot) []workv1alpha2.TargetCluster {
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

func mergeLabel(workload *unstructured.Unstructured, binding metav1.Object, scope apiextensionsv1.ResourceScope) map[string]string {
	var workLabel = make(map[string]string)
	if scope == apiextensionsv1.NamespaceScoped {
		bindingID := util.GetLabelValue(binding.GetLabels(), workv1alpha2.ResourceBindingPermanentIDLabel)
		util.MergeLabel(workload, workv1alpha2.ResourceBindingPermanentIDLabel, bindingID)
		workLabel[workv1alpha2.ResourceBindingPermanentIDLabel] = bindingID
	} else {
		bindingID := util.GetLabelValue(binding.GetLabels(), workv1alpha2.ClusterResourceBindingPermanentIDLabel)
		util.MergeLabel(workload, workv1alpha2.ClusterResourceBindingPermanentIDLabel, bindingID)
		workLabel[workv1alpha2.ClusterResourceBindingPermanentIDLabel] = bindingID
	}
	return workLabel
}

func mergeAnnotations(workload *unstructured.Unstructured, binding metav1.Object, scope apiextensionsv1.ResourceScope) map[string]string {
	annotations := make(map[string]string)
	if workload.GetGeneration() > 0 {
		util.MergeAnnotation(workload, workv1alpha2.ResourceTemplateGenerationAnnotationKey, strconv.FormatInt(workload.GetGeneration(), 10))
	}

	if scope == apiextensionsv1.NamespaceScoped {
		util.MergeAnnotation(workload, workv1alpha2.ResourceBindingNamespaceAnnotationKey, binding.GetNamespace())
		util.MergeAnnotation(workload, workv1alpha2.ResourceBindingNameAnnotationKey, binding.GetName())
		annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey] = binding.GetNamespace()
		annotations[workv1alpha2.ResourceBindingNameAnnotationKey] = binding.GetName()
	} else {
		util.MergeAnnotation(workload, workv1alpha2.ClusterResourceBindingAnnotationKey, binding.GetName())
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

// mergeConflictResolution determine the conflictResolution annotation of Work: preferentially inherit from RT, then RB
func mergeConflictResolution(workload *unstructured.Unstructured, conflictResolutionInBinding policyv1alpha1.ConflictResolution,
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
	var targetClusters []workv1alpha2.TargetCluster
	completions, found, err := unstructured.NestedInt64(workload.Object, util.SpecField, util.CompletionsField)
	if err != nil {
		return nil, err
	}

	if found {
		targetClusters = helper.SpreadReplicasByTargetClusters(int32(completions), clusters, nil) // #nosec G115: integer overflow conversion int64 -> int32
	}

	return targetClusters, nil
}

func needReviseReplicas(replicas int32) bool {
	return replicas > 0
}

func needReviseJobCompletions(replicas int32, placement *policyv1alpha1.Placement) bool {
	return replicas > 0 && placement != nil && placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDivided
}

func shouldSuspendDispatching(suspension *workv1alpha2.Suspension, targetCluster workv1alpha2.TargetCluster) bool {
	if suspension == nil {
		return false
	}

	suspendDispatching := ptr.Deref(suspension.Dispatching, false)

	if !suspendDispatching && suspension.DispatchingOnClusters != nil {
		for _, cluster := range suspension.DispatchingOnClusters.ClusterNames {
			if cluster == targetCluster.Name {
				suspendDispatching = true
				break
			}
		}
	}
	return suspendDispatching
}
