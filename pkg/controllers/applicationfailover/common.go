/*
Copyright 2023 The Karmada Authors.

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

package applicationfailover

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type workloadUnhealthyMap struct {
	sync.RWMutex
	// key is the NamespacedName of the binding
	// value is also a map. Its key is the cluster where the unhealthy workload resides.
	// Its value is the time when the unhealthy state was first observed.
	workloadUnhealthy map[types.NamespacedName]map[string]metav1.Time
}

func newWorkloadUnhealthyMap() *workloadUnhealthyMap {
	return &workloadUnhealthyMap{
		workloadUnhealthy: make(map[types.NamespacedName]map[string]metav1.Time),
	}
}

func (m *workloadUnhealthyMap) delete(key types.NamespacedName) {
	m.Lock()
	defer m.Unlock()
	delete(m.workloadUnhealthy, key)
}

func (m *workloadUnhealthyMap) hasWorkloadBeenUnhealthy(key types.NamespacedName, cluster string) bool {
	m.RLock()
	defer m.RUnlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	if unhealthyClusters == nil {
		return false
	}

	_, exist := unhealthyClusters[cluster]
	return exist
}

func (m *workloadUnhealthyMap) setTimeStamp(key types.NamespacedName, cluster string) {
	m.Lock()
	defer m.Unlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	if unhealthyClusters == nil {
		unhealthyClusters = make(map[string]metav1.Time)
	}

	unhealthyClusters[cluster] = metav1.Now()
	m.workloadUnhealthy[key] = unhealthyClusters
}

func (m *workloadUnhealthyMap) getTimeStamp(key types.NamespacedName, cluster string) metav1.Time {
	m.RLock()
	defer m.RUnlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	return unhealthyClusters[cluster]
}

func (m *workloadUnhealthyMap) deleteIrrelevantClusters(key types.NamespacedName, allClusters sets.Set[string], healthyClusters []string) {
	m.Lock()
	defer m.Unlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	if unhealthyClusters == nil {
		return
	}
	for cluster := range unhealthyClusters {
		if !allClusters.Has(cluster) {
			delete(unhealthyClusters, cluster)
		}
	}
	for _, cluster := range healthyClusters {
		delete(unhealthyClusters, cluster)
	}

	m.workloadUnhealthy[key] = unhealthyClusters
}

// workloadUnhealthyTimestampsAnnotation persists per-cluster unhealthy detection
// timestamps so that the toleration timer survives controller restarts.
const workloadUnhealthyTimestampsAnnotation = "work.karmada.io/unhealthy-first-detected"

// restoreFromAnnotation populates the in-memory map from persisted annotation data.
// It is called at the beginning of syncBinding to recover timestamps after a controller restart.
// If the in-memory map already has entries for the given key, this is a no-op.
func (m *workloadUnhealthyMap) restoreFromAnnotation(key types.NamespacedName, annotations map[string]string) {
	m.Lock()
	defer m.Unlock()

	if _, exists := m.workloadUnhealthy[key]; exists {
		return
	}

	value := annotations[workloadUnhealthyTimestampsAnnotation]
	if value == "" {
		return
	}

	var timestamps map[string]metav1.Time
	if err := json.Unmarshal([]byte(value), &timestamps); err != nil {
		klog.V(4).InfoS("Failed to unmarshal unhealthy timestamps annotation", "err", err)
		return
	}
	if len(timestamps) > 0 {
		m.workloadUnhealthy[key] = timestamps
	}
}

// getTimestampsSnapshot returns a shallow copy of the per-cluster unhealthy timestamps
// for the given binding key. Returns nil if no entries exist.
func (m *workloadUnhealthyMap) getTimestampsSnapshot(key types.NamespacedName) map[string]metav1.Time {
	m.RLock()
	defer m.RUnlock()

	src := m.workloadUnhealthy[key]
	if len(src) == 0 {
		return nil
	}
	result := make(map[string]metav1.Time, len(src))
	for k, v := range src {
		result[k] = v
	}
	return result
}

// marshalUnhealthyTimestamps serializes per-cluster unhealthy detection timestamps to JSON.
// Returns an empty string when the input map is empty, or an error if JSON encoding fails.
func marshalUnhealthyTimestamps(timestamps map[string]metav1.Time) (string, error) {
	if len(timestamps) == 0 {
		return "", nil
	}
	data, err := json.Marshal(timestamps)
	if err != nil {
		return "", fmt.Errorf("failed to marshal unhealthy timestamps: %w", err)
	}
	return string(data), nil
}

// persistUnhealthyTimestamps writes the current in-memory unhealthy detection timestamps
// to the binding's annotation via a merge patch. It skips the patch if the annotation
// value is already up to date. Called at the end of each syncBinding reconciliation.
func persistUnhealthyTimestamps(ctx context.Context, c client.Client, m *workloadUnhealthyMap, key types.NamespacedName, obj client.Object) error {
	timestamps := m.getTimestampsSnapshot(key)
	newValue, err := marshalUnhealthyTimestamps(timestamps)
	if err != nil {
		return err
	}
	oldValue := obj.GetAnnotations()[workloadUnhealthyTimestampsAnnotation]

	if newValue == oldValue {
		return nil
	}

	objPatch := client.MergeFrom(obj)
	modifiedObj := obj.DeepCopyObject().(client.Object)
	annotations := modifiedObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if newValue == "" {
		delete(annotations, workloadUnhealthyTimestampsAnnotation)
	} else {
		annotations[workloadUnhealthyTimestampsAnnotation] = newValue
	}
	modifiedObj.SetAnnotations(annotations)
	return c.Patch(ctx, modifiedObj, objPatch)
}

// distinguishUnhealthyClustersWithOthers distinguishes clusters which is in the unHealthy state(not in the process of eviction) with others.
func distinguishUnhealthyClustersWithOthers(aggregatedStatusItems []workv1alpha2.AggregatedStatusItem, resourceBindingSpec workv1alpha2.ResourceBindingSpec) ([]string, []string) {
	var unhealthyClusters, others []string
	for _, aggregatedStatusItem := range aggregatedStatusItems {
		cluster := aggregatedStatusItem.ClusterName

		if aggregatedStatusItem.Health == workv1alpha2.ResourceUnhealthy && !resourceBindingSpec.ClusterInGracefulEvictionTasks(cluster) {
			unhealthyClusters = append(unhealthyClusters, cluster)
		}

		if aggregatedStatusItem.Health == workv1alpha2.ResourceHealthy || aggregatedStatusItem.Health == workv1alpha2.ResourceUnknown {
			others = append(others, cluster)
		}
	}

	return unhealthyClusters, others
}

func getClusterNamesFromTargetClusters(targetClusters []workv1alpha2.TargetCluster) []string {
	if targetClusters == nil {
		return nil
	}

	clusters := make([]string, 0, len(targetClusters))
	for _, targetCluster := range targetClusters {
		clusters = append(clusters, targetCluster.Name)
	}
	return clusters
}

func buildTaskOptions(failoverBehavior *policyv1alpha1.ApplicationFailoverBehavior, aggregatedStatus []workv1alpha2.AggregatedStatusItem, cluster, producer string, clustersBeforeFailover []string) ([]workv1alpha2.Option, error) {
	var taskOpts []workv1alpha2.Option
	taskOpts = append(taskOpts, workv1alpha2.WithProducer(producer))
	taskOpts = append(taskOpts, workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure))
	taskOpts = append(taskOpts, workv1alpha2.WithPurgeMode(failoverBehavior.PurgeMode))

	if features.FeatureGate.Enabled(features.StatefulFailoverInjection) {
		if failoverBehavior.StatePreservation != nil && len(failoverBehavior.StatePreservation.Rules) != 0 {
			targetStatusItem, exist := helper.FindTargetStatusItemByCluster(aggregatedStatus, cluster)
			if !exist || targetStatusItem.Status == nil || targetStatusItem.Status.Raw == nil {
				return nil, fmt.Errorf("the application status has not yet been collected from Cluster(%s)", cluster)
			}
			preservedLabelState, err := helper.BuildPreservedLabelState(failoverBehavior.StatePreservation, targetStatusItem.Status.Raw)
			if err != nil {
				return nil, err
			}
			if preservedLabelState != nil {
				taskOpts = append(taskOpts, workv1alpha2.WithPreservedLabelState(preservedLabelState))
				taskOpts = append(taskOpts, workv1alpha2.WithClustersBeforeFailover(clustersBeforeFailover))
			}
		}
	}

	switch failoverBehavior.PurgeMode {
	//nolint:staticcheck
	// disable `deprecation` check for backward compatibility.
	case policyv1alpha1.Graciously, policyv1alpha1.PurgeModeGracefully:
		if features.FeatureGate.Enabled(features.GracefulEviction) {
			taskOpts = append(taskOpts, workv1alpha2.WithGracePeriodSeconds(failoverBehavior.GracePeriodSeconds))
		} else {
			err := fmt.Errorf("GracefulEviction featureGate must be enabled when purgeMode is %s", failoverBehavior.PurgeMode)
			klog.Error(err)
			return nil, err
		}
	case policyv1alpha1.Never:
		taskOpts = append(taskOpts, workv1alpha2.WithSuppressDeletion(ptr.To[bool](true)))
	}

	return taskOpts, nil
}
