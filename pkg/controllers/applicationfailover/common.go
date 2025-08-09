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
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
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

func buildPreservedLabelState(statePreservation *policyv1alpha1.StatePreservation, rawStatus []byte) (map[string]string, error) {
	results := make(map[string]string, len(statePreservation.Rules))
	for _, rule := range statePreservation.Rules {
		value, err := parseJSONValue(rawStatus, rule.JSONPath)
		if err != nil {
			klog.ErrorS(err, "Failed to parse value with jsonPath from status",
				"jsonPath", rule.JSONPath,
				"status", string(rawStatus))
			return nil, err
		}
		results[rule.AliasLabelName] = value
	}

	return results, nil
}

func parseJSONValue(rawStatus []byte, jsonPath string) (string, error) {
	template := jsonPath
	j := jsonpath.New(jsonPath)
	j.AllowMissingKeys(false)
	err := j.Parse(template)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	unmarshalled := make(map[string]interface{})
	_ = json.Unmarshal(rawStatus, &unmarshalled)
	err = j.Execute(buf, unmarshalled)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func findTargetStatusItemByCluster(aggregatedStatusItems []workv1alpha2.AggregatedStatusItem, cluster string) (workv1alpha2.AggregatedStatusItem, bool) {
	if len(aggregatedStatusItems) == 0 {
		return workv1alpha2.AggregatedStatusItem{}, false
	}

	for index, statusItem := range aggregatedStatusItems {
		if statusItem.ClusterName == cluster {
			return aggregatedStatusItems[index], true
		}
	}

	return workv1alpha2.AggregatedStatusItem{}, false
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
			targetStatusItem, exist := findTargetStatusItemByCluster(aggregatedStatus, cluster)
			if !exist || targetStatusItem.Status == nil || targetStatusItem.Status.Raw == nil {
				return nil, fmt.Errorf("the application status has not yet been collected from Cluster(%s)", cluster)
			}
			preservedLabelState, err := buildPreservedLabelState(failoverBehavior.StatePreservation, targetStatusItem.Status.Raw)
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
	case policyv1alpha1.Graciously:
		if features.FeatureGate.Enabled(features.GracefulEviction) {
			taskOpts = append(taskOpts, workv1alpha2.WithGracePeriodSeconds(failoverBehavior.GracePeriodSeconds))
		} else {
			err := fmt.Errorf("GracefulEviction featureGate must be enabled when purgeMode is %s", policyv1alpha1.Graciously)
			klog.Error(err)
			return nil, err
		}
	case policyv1alpha1.Never:
		taskOpts = append(taskOpts, workv1alpha2.WithSuppressDeletion(ptr.To[bool](true)))
	}

	return taskOpts, nil
}
