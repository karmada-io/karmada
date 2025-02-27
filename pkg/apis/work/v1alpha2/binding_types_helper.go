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

package v1alpha2

import policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"

// TaskOptions represents options for GracefulEvictionTasks.
type TaskOptions struct {
	purgeMode              policyv1alpha1.PurgeMode
	producer               string
	reason                 string
	message                string
	gracePeriodSeconds     *int32
	suppressDeletion       *bool
	preservedLabelState    map[string]string
	clustersBeforeFailover []string
}

// Option configures a TaskOptions
type Option func(*TaskOptions)

// NewTaskOptions builds a TaskOptions
func NewTaskOptions(opts ...Option) *TaskOptions {
	options := TaskOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	return &options
}

// WithPurgeMode sets the purgeMode for TaskOptions
func WithPurgeMode(purgeMode policyv1alpha1.PurgeMode) Option {
	return func(o *TaskOptions) {
		o.purgeMode = purgeMode
	}
}

// WithProducer sets the producer for TaskOptions
func WithProducer(producer string) Option {
	return func(o *TaskOptions) {
		o.producer = producer
	}
}

// WithReason sets the reason for TaskOptions
func WithReason(reason string) Option {
	return func(o *TaskOptions) {
		o.reason = reason
	}
}

// WithMessage sets the message for TaskOptions
func WithMessage(message string) Option {
	return func(o *TaskOptions) {
		o.message = message
	}
}

// WithGracePeriodSeconds sets the gracePeriodSeconds for TaskOptions
func WithGracePeriodSeconds(gracePeriodSeconds *int32) Option {
	return func(o *TaskOptions) {
		o.gracePeriodSeconds = gracePeriodSeconds
	}
}

// WithSuppressDeletion sets the suppressDeletion for TaskOptions
func WithSuppressDeletion(suppressDeletion *bool) Option {
	return func(o *TaskOptions) {
		o.suppressDeletion = suppressDeletion
	}
}

// WithPreservedLabelState sets the preservedLabelState for TaskOptions
func WithPreservedLabelState(preservedLabelState map[string]string) Option {
	return func(o *TaskOptions) {
		o.preservedLabelState = preservedLabelState
	}
}

// WithClustersBeforeFailover sets the clustersBeforeFailover for TaskOptions
func WithClustersBeforeFailover(clustersBeforeFailover []string) Option {
	return func(o *TaskOptions) {
		o.clustersBeforeFailover = clustersBeforeFailover
	}
}

// TargetContains checks if specific cluster present on the target list.
func (s *ResourceBindingSpec) TargetContains(name string) bool {
	for i := range s.Clusters {
		if s.Clusters[i].Name == name {
			return true
		}
	}

	return false
}

// ClusterInGracefulEvictionTasks checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
func (s *ResourceBindingSpec) ClusterInGracefulEvictionTasks(name string) bool {
	for _, task := range s.GracefulEvictionTasks {
		if task.FromCluster == name {
			return true
		}
	}
	return false
}

// AssignedReplicasForCluster returns assigned replicas for specific cluster.
func (s *ResourceBindingSpec) AssignedReplicasForCluster(targetCluster string) int32 {
	for i := range s.Clusters {
		if s.Clusters[i].Name == targetCluster {
			return s.Clusters[i].Replicas
		}
	}

	return 0
}

// RemoveCluster removes specific cluster from the target list.
// This function no-opts if cluster not exist.
func (s *ResourceBindingSpec) RemoveCluster(name string) {
	var i int

	for i = 0; i < len(s.Clusters); i++ {
		if s.Clusters[i].Name == name {
			break
		}
	}

	// not found, do nothing
	if i >= len(s.Clusters) {
		return
	}

	s.Clusters = append(s.Clusters[:i], s.Clusters[i+1:]...)
}

// GracefulEvictCluster removes specific cluster from the target list in a graceful way by
// building a graceful eviction task.
// This function no-opts if the cluster does not exist.
func (s *ResourceBindingSpec) GracefulEvictCluster(name string, options *TaskOptions) {
	// find the cluster index
	var i int
	for i = 0; i < len(s.Clusters); i++ {
		if s.Clusters[i].Name == name {
			break
		}
	}
	// not found, do nothing
	if i >= len(s.Clusters) {
		return
	}

	evictCluster := s.Clusters[i]

	// remove the target cluster from scheduling result
	s.Clusters = append(s.Clusters[:i], s.Clusters[i+1:]...)

	// skip if the target cluster already in the task list
	if s.ClusterInGracefulEvictionTasks(evictCluster.Name) {
		return
	}

	// build eviction task
	evictingCluster := evictCluster.DeepCopy()
	evictionTask := GracefulEvictionTask{
		FromCluster:            evictingCluster.Name,
		PurgeMode:              options.purgeMode,
		Reason:                 options.reason,
		Message:                options.message,
		Producer:               options.producer,
		GracePeriodSeconds:     options.gracePeriodSeconds,
		SuppressDeletion:       options.suppressDeletion,
		PreservedLabelState:    options.preservedLabelState,
		ClustersBeforeFailover: options.clustersBeforeFailover,
	}
	if evictingCluster.Replicas > 0 {
		evictionTask.Replicas = &evictingCluster.Replicas
	}
	s.GracefulEvictionTasks = append(s.GracefulEvictionTasks, evictionTask)
}

// SchedulingSuspended tells if the scheduling of ResourceBinding or
// ClusterResourceBinding is suspended.
func (s *ResourceBindingSpec) SchedulingSuspended() bool {
	if s == nil || s.Suspension == nil || s.Suspension.Scheduling == nil {
		return false
	}

	return *s.Suspension.Scheduling
}

// SchedulePriorityValue returns the scheduling priority declared
// by '.spec.SchedulePriority.Priority'.
func (s *ResourceBindingSpec) SchedulePriorityValue() int32 {
	if s.SchedulePriority == nil {
		return 0
	}
	return s.SchedulePriority.Priority
}
