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
	"sync"
	"time"

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

const (
	// requeueIntervalForDirectlyPurge is the requeue interval for binding when there are works in clusters with PurgeMode 'Directly'.
	requeueIntervalForDirectlyPurge = 5 * time.Second
)

// workTask represents a task to create/update a Work object
type workTask struct {
	workMeta      metav1.ObjectMeta
	workload      *unstructured.Unstructured
	suspendDisp   bool
	preserveOnDel bool
}

// prepareResult holds the result of preparing a single work task
type prepareResult struct {
	index int
	task  workTask
	err   error
}

// ensureWork ensure Work to be created or updated.
// Optimized with parallel processing for both preparation and execution phases.
func ensureWork(
	ctx context.Context, c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter, workload *unstructured.Unstructured,
	overrideManager overridemanager.OverrideManager, binding metav1.Object, scope apiextensionsv1.ResourceScope,
) error {
	bindingSpec := getBindingSpec(binding, scope)
	targetClusters := mergeTargetClusters(bindingSpec.Clusters, bindingSpec.RequiredBy)

	// Fast path: no target clusters
	if len(targetClusters) == 0 {
		return nil
	}

	var jobCompletions []workv1alpha2.TargetCluster
	var err error
	if workload.GetKind() == util.JobKind && needReviseJobCompletions(bindingSpec.Replicas, bindingSpec.Placement) {
		jobCompletions, err = divideReplicasByJobCompletions(workload, targetClusters)
		if err != nil {
			return err
		}
	}

	numClusters := len(targetClusters)

	// Single cluster: use sequential processing (no goroutine overhead)
	if numClusters == 1 {
		return ensureWorkSequential(ctx, c, resourceInterpreter, workload, overrideManager, binding, scope, bindingSpec, targetClusters, jobCompletions)
	}

	// Multiple clusters: parallel preparation and execution
	return ensureWorkParallel(ctx, c, resourceInterpreter, workload, overrideManager, binding, scope, bindingSpec, targetClusters, jobCompletions)
}

// ensureWorkSequential handles single cluster case without goroutine overhead
func ensureWorkSequential(
	ctx context.Context, c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter, workload *unstructured.Unstructured,
	overrideManager overridemanager.OverrideManager, binding metav1.Object, scope apiextensionsv1.ResourceScope,
	bindingSpec workv1alpha2.ResourceBindingSpec, targetClusters []workv1alpha2.TargetCluster, jobCompletions []workv1alpha2.TargetCluster,
) error {
	targetCluster := targetClusters[0]
	clonedWorkload := workload.DeepCopy()

	task, err := prepareWorkTask(PrepareWorkTaskArgs{
		ResourceInterpreter: resourceInterpreter,
		ClonedWorkload:      clonedWorkload,
		OverrideManager:     overrideManager,
		Binding:             binding,
		Scope:               scope,
		BindingSpec:         bindingSpec,
		TargetCluster:       targetCluster,
		ClusterIndex:        0,
		JobCompletions:      jobCompletions,
		TotalClusters:       len(targetClusters),
	})
	if err != nil {
		return err
	}

	return ctrlutil.CreateOrUpdateWork(ctx, c, task.workMeta, task.workload,
		ctrlutil.WithSuspendDispatching(task.suspendDisp),
		ctrlutil.WithPreserveResourcesOnDeletion(task.preserveOnDel))
}

// ensureWorkParallel handles multiple clusters with parallel preparation and execution
func ensureWorkParallel(
	ctx context.Context, c client.Client, resourceInterpreter resourceinterpreter.ResourceInterpreter, workload *unstructured.Unstructured,
	overrideManager overridemanager.OverrideManager, binding metav1.Object, scope apiextensionsv1.ResourceScope,
	bindingSpec workv1alpha2.ResourceBindingSpec, targetClusters []workv1alpha2.TargetCluster, jobCompletions []workv1alpha2.TargetCluster,
) error {
	numClusters := len(targetClusters)

	// Phase 1: Parallel preparation
	resultCh := make(chan prepareResult, numClusters)
	var prepareWg sync.WaitGroup

	for i := range targetClusters {
		prepareWg.Add(1)
		go func(idx int, tc workv1alpha2.TargetCluster) {
			defer prepareWg.Done()

			// Each goroutine gets its own deep copy
			clonedWorkload := workload.DeepCopy()

			task, err := prepareWorkTask(PrepareWorkTaskArgs{
				ResourceInterpreter: resourceInterpreter,
				ClonedWorkload:      clonedWorkload,
				OverrideManager:     overrideManager,
				Binding:             binding,
				Scope:               scope,
				BindingSpec:         bindingSpec,
				TargetCluster:       tc,
				ClusterIndex:        idx,
				JobCompletions:      jobCompletions,
				TotalClusters:       numClusters,
			})
			resultCh <- prepareResult{index: idx, task: task, err: err}
		}(i, targetClusters[i])
	}

	// Wait for all preparations to complete, then close channel
	go func() {
		prepareWg.Wait()
		close(resultCh)
	}()

	// Collect preparation results
	tasks := make([]workTask, numClusters)
	var prepareErrs []error
	validCount := 0

	for result := range resultCh {
		if result.err != nil {
			prepareErrs = append(prepareErrs, result.err)
			continue
		}
		tasks[result.index] = result.task
		validCount++
	}

	// If no valid tasks, return prepare errors
	if validCount == 0 {
		return errors.NewAggregate(prepareErrs)
	}

	// Phase 2: Parallel execution
	errCh := make(chan error, validCount)
	var execWg sync.WaitGroup

	for i := range tasks {
		// Skip empty tasks (failed preparation)
		if tasks[i].workload == nil {
			continue
		}

		execWg.Add(1)
		go func(task workTask) {
			defer execWg.Done()
			if err := ctrlutil.CreateOrUpdateWork(ctx, c, task.workMeta, task.workload,
				ctrlutil.WithSuspendDispatching(task.suspendDisp),
				ctrlutil.WithPreserveResourcesOnDeletion(task.preserveOnDel)); err != nil {
				errCh <- err
			}
		}(tasks[i])
	}

	execWg.Wait()
	close(errCh)

	// Collect execution errors
	var executeErrs []error
	for err := range errCh {
		executeErrs = append(executeErrs, err)
	}

	// Combine all errors
	allErrs := append(prepareErrs, executeErrs...)
	if len(allErrs) > 0 {
		return errors.NewAggregate(allErrs)
	}
	return nil
}

// PrepareWorkTaskArgs contains all arguments for prepareWorkTask function.
// This struct encapsulates the 10 parameters to improve readability and maintainability.
type PrepareWorkTaskArgs struct {
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	ClonedWorkload      *unstructured.Unstructured
	OverrideManager     overridemanager.OverrideManager
	Binding             metav1.Object
	Scope               apiextensionsv1.ResourceScope
	BindingSpec         workv1alpha2.ResourceBindingSpec
	TargetCluster       workv1alpha2.TargetCluster
	ClusterIndex        int
	JobCompletions      []workv1alpha2.TargetCluster
	TotalClusters       int
}

// prepareWorkTask prepares a single work task for a target cluster.
// This function is safe to call concurrently as long as each call has its own clonedWorkload.
func prepareWorkTask(args PrepareWorkTaskArgs) (workTask, error) {
	clonedWorkload := args.ClonedWorkload
	workNamespace := names.GenerateExecutionSpaceName(args.TargetCluster.Name)

	// When syncing workloads to member clusters, the controller MUST strictly adhere to the scheduling results
	// specified in bindingSpec.Clusters for replica allocation, rather than using the replicas declared in the
	// workload's resource template.
	if args.BindingSpec.IsWorkload() {
		if args.ResourceInterpreter.HookEnabled(clonedWorkload.GroupVersionKind(), configv1alpha1.InterpreterOperationReviseReplica) {
			var err error
			clonedWorkload, err = args.ResourceInterpreter.ReviseReplica(clonedWorkload, int64(args.TargetCluster.Replicas))
			if err != nil {
				klog.ErrorS(err, "Failed to revise replica for workload in cluster.",
					"workloadKind", clonedWorkload.GetKind(), "workloadNamespace", clonedWorkload.GetNamespace(),
					"workloadName", clonedWorkload.GetName(), "cluster", args.TargetCluster.Name)
				return workTask{}, err
			}
		}
	}

	// Set allocated completions for Job only when the '.spec.completions' field not omitted from resource template.
	if len(args.JobCompletions) > 0 && args.ClusterIndex < len(args.JobCompletions) {
		if err := helper.ApplyReplica(clonedWorkload, int64(args.JobCompletions[args.ClusterIndex].Replicas), util.CompletionsField); err != nil {
			klog.ErrorS(err, "Failed to apply Completions for workload in cluster.",
				"workloadKind", clonedWorkload.GetKind(), "workloadNamespace", clonedWorkload.GetNamespace(),
				"workloadName", clonedWorkload.GetName(), "cluster", args.TargetCluster.Name)
			return workTask{}, err
		}
	}

	// We should call ApplyOverridePolicies last, as override rules have the highest priority
	cops, ops, err := args.OverrideManager.ApplyOverridePolicies(clonedWorkload, args.TargetCluster.Name)
	if err != nil {
		klog.ErrorS(err, "Failed to apply overrides for workload in cluster.",
			"workloadKind", clonedWorkload.GetKind(), "workloadNamespace", clonedWorkload.GetNamespace(),
			"workloadName", clonedWorkload.GetName(), "cluster", args.TargetCluster.Name)
		return workTask{}, err
	}

	workLabel := mergeLabel(clonedWorkload, args.Binding, args.Scope)

	annotations := mergeAnnotations(clonedWorkload, args.Binding, args.Scope)
	annotations = mergeConflictResolution(clonedWorkload, args.BindingSpec.ConflictResolution, annotations)
	annotations, err = RecordAppliedOverrides(cops, ops, annotations)
	if err != nil {
		klog.ErrorS(err, "Failed to record appliedOverrides in cluster.", "cluster", args.TargetCluster.Name)
		return workTask{}, err
	}

	if features.FeatureGate.Enabled(features.StatefulFailoverInjection) {
		clonedWorkload = injectReservedLabelState(args.BindingSpec, args.TargetCluster, clonedWorkload, args.TotalClusters)
	}

	return workTask{
		workMeta: metav1.ObjectMeta{
			Name:        names.GenerateWorkName(clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace()),
			Namespace:   workNamespace,
			Finalizers:  []string{util.ExecutionControllerFinalizer},
			Labels:      workLabel,
			Annotations: annotations,
		},
		workload:      clonedWorkload,
		suspendDisp:   shouldSuspendDispatching(args.BindingSpec.Suspension, args.TargetCluster),
		preserveOnDel: ptr.Deref(args.BindingSpec.PreserveResourcesOnDeletion, false),
	}, nil
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
//  3. The injection operation is performed only when PurgeMode is set to Immediately or Directly.
func injectReservedLabelState(bindingSpec workv1alpha2.ResourceBindingSpec, moveToCluster workv1alpha2.TargetCluster, workload *unstructured.Unstructured, clustersLen int) *unstructured.Unstructured {
	if clustersLen > 1 {
		return workload
	}

	if len(bindingSpec.GracefulEvictionTasks) == 0 {
		return workload
	}
	targetEvictionTask := bindingSpec.GracefulEvictionTasks[len(bindingSpec.GracefulEvictionTasks)-1]

	//nolint:staticcheck
	// disable `deprecation` check for backward compatibility.
	if targetEvictionTask.PurgeMode != policyv1alpha1.Immediately &&
		targetEvictionTask.PurgeMode != policyv1alpha1.PurgeModeDirectly {
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
		targetClusters = helper.SpreadReplicasByTargetClusters(int32(completions), clusters, nil, workload.GetUID()) // #nosec G115: integer overflow conversion int64 -> int32
	}

	return targetClusters, nil
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
