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

// Note: This file is a refactored version of the original test file,
// structured to support the new plugin-based eviction logic.

package gracefuleviction

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/controllers/gracefuleviction/evictplugins"
)

// mockEvictorPlugin is a mock implementation of EvictorPlugin for testing.
type mockEvictorPlugin struct {
	name        string
	canEvictRB  bool
	canEvictCRB bool
}

// Name returns the mock plugin's name.
func (m *mockEvictorPlugin) Name() string { return m.name }

// CanEvictRB returns the mock value.
func (m *mockEvictorPlugin) CanEvictRB(_ context.Context, _ *workv1alpha2.GracefulEvictionTask, _ *workv1alpha2.ResourceBinding) bool {
	return m.canEvictRB
}

// CanEvictCRB returns the mock value.
func (m *mockEvictorPlugin) CanEvictCRB(_ context.Context, _ *workv1alpha2.GracefulEvictionTask, _ *workv1alpha2.ClusterResourceBinding) bool {
	return m.canEvictCRB
}

// Self-register mock plugins for testing.
// This init function will be called by the Go testing framework before tests are run,
// making the mock plugins available to the PluginManager.
func init() {
	plugins.RegisterPlugin("mock-allow", func() (plugins.EvictorPlugin, error) {
		return &mockEvictorPlugin{name: "mock-allow", canEvictRB: true, canEvictCRB: true}, nil
	})
	plugins.RegisterPlugin("mock-deny", func() (plugins.EvictorPlugin, error) {
		return &mockEvictorPlugin{name: "mock-deny", canEvictRB: false, canEvictCRB: false}, nil
	})
}

// Test_isTaskFinishedForRB is the test suite for isTaskFinishedForRB.
func Test_isTaskFinishedForRB(t *testing.T) {
	// --- Test Setup ---
	timeNow := metav1.Now()
	timeout := 3 * time.Minute

	// Create mock plugin managers for different scenarios.
	pluginAllowMgr, _ := plugins.NewManager([]string{"mock-allow"})
	pluginDenyMgr, _ := plugins.NewManager([]string{"mock-deny"})

	// --- Test Cases ---
	tests := []struct {
		name     string
		task     *workv1alpha2.GracefulEvictionTask
		opt      assessmentOptionRB
		expected bool // The function now returns a simple boolean.
	}{
		{
			name:     "task with SuppressDeletion=true should not be finished",
			task:     &workv1alpha2.GracefulEvictionTask{SuppressDeletion: ptr.To(true)},
			opt:      assessmentOptionRB{binding: &workv1alpha2.ResourceBinding{}, pluginMgr: pluginDenyMgr, timeout: timeout},
			expected: false,
		},
		{
			name:     "task with SuppressDeletion=false should be finished",
			task:     &workv1alpha2.GracefulEvictionTask{SuppressDeletion: ptr.To(false)},
			opt:      assessmentOptionRB{binding: &workv1alpha2.ResourceBinding{}, pluginMgr: pluginDenyMgr, timeout: timeout},
			expected: true,
		},
		{
			name:     "task exceeds timeout should be finished",
			task:     &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &metav1.Time{Time: timeNow.Add(-4 * time.Minute)}},
			opt:      assessmentOptionRB{binding: &workv1alpha2.ResourceBinding{}, pluginMgr: pluginDenyMgr, timeout: timeout},
			expected: true,
		},
		// --- New tests for the plugin logic ---
		{
			name: "plugin allows but binding not scheduled yet, should not be finished",
			task: &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &timeNow},
			opt: assessmentOptionRB{
				// Generation is 2, but scheduler has only observed 1, so hasScheduled is false.
				binding:   &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1}},
				pluginMgr: pluginAllowMgr,
				timeout:   timeout,
			},
			expected: false,
		},
		{
			name: "binding scheduled but plugin denies, should not be finished",
			task: &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &timeNow},
			opt: assessmentOptionRB{
				// Generation and observed generation match, so hasScheduled is true.
				binding:   &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 2}},
				pluginMgr: pluginDenyMgr, // Using the manager that will deny.
				timeout:   timeout,
			},
			expected: false,
		},
		{
			name: "binding scheduled and plugin allows, should be finished",
			task: &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &timeNow},
			opt: assessmentOptionRB{
				// Generation and observed generation match, so hasScheduled is true.
				binding:   &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 2}},
				pluginMgr: pluginAllowMgr, // Using the manager that will allow.
				timeout:   timeout,
			},
			expected: true,
		},
	}

	// --- Test Execution ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test.
			result := isTaskFinishedForRB(context.Background(), tt.task, tt.opt)
			// Assert the result.
			if result != tt.expected {
				t.Errorf("isTaskFinishedForRB() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test_assessEvictionTasksForRB is the test suite for assessEvictionTasksForRB.
// It focuses on testing the orchestration logic, such as task iteration,
// timestamping new tasks, and correctly sorting tasks into kept/evicted lists.
func Test_assessEvictionTasksForRB(t *testing.T) {
	// --- Test Setup ---
	timeNow := metav1.Now()
	defaultTimeout := 3 * time.Minute

	pluginAllowMgr, _ := plugins.NewManager([]string{"mock-allow"})
	pluginDenyMgr, _ := plugins.NewManager([]string{"mock-deny"})

	// --- Test Cases ---
	tests := []struct {
		name                    string
		opt                     assessmentOptionRB
		expectedKeptTasks       []string // We will check the cluster names of kept tasks for simplicity.
		expectedEvictedClusters []string
	}{
		{
			name: "no tasks to assess",
			opt: assessmentOptionRB{
				binding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{},
					},
				},
				pluginMgr: pluginDenyMgr,
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{},
			expectedEvictedClusters: nil,
		},
		{
			name: "a new task should be kept and have its timestamp set",
			opt: assessmentOptionRB{
				binding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
							{FromCluster: "member1"}, // A new task with zero CreationTimestamp
						},
					},
				},
				pluginMgr: pluginDenyMgr,
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{"member1"},
			expectedEvictedClusters: nil,
		},
		{
			name: "one task timed out, another not",
			opt: assessmentOptionRB{
				binding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Generation: 1},
					Status:     workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1},
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
							{FromCluster: "member1", CreationTimestamp: &metav1.Time{Time: timeNow.Add(-1 * time.Minute)}}, // Should be kept
							{FromCluster: "member2", CreationTimestamp: &metav1.Time{Time: timeNow.Add(-5 * time.Minute)}}, // Should be evicted
						},
					},
				},
				pluginMgr: pluginDenyMgr,
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{"member1"},
			expectedEvictedClusters: []string{"member2"},
		},
		{
			name: "a task is evicted early by a plugin",
			opt: assessmentOptionRB{
				binding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Generation: 1},
					Status:     workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1}, // hasScheduled is true
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
							{FromCluster: "member1", CreationTimestamp: &timeNow}, // Not timed out, but plugin will allow
						},
					},
				},
				pluginMgr: pluginAllowMgr, // Using the manager that allows eviction
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{},
			expectedEvictedClusters: []string{"member1"},
		},
		{
			name: "a task with custom GracePeriodSeconds should override default timeout",
			opt: assessmentOptionRB{
				binding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Generation: 1},
					Status:     workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1},
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
							// This task was created 2 minutes ago, default timeout is 3 minutes (not expired).
							// But its custom grace period is 1 minute (60s), so it is expired.
							{FromCluster: "member1", CreationTimestamp: &metav1.Time{Time: timeNow.Add(-2 * time.Minute)}, GracePeriodSeconds: ptr.To[int32](60)},
						},
					},
				},
				pluginMgr: pluginDenyMgr,
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{},
			expectedEvictedClusters: []string{"member1"},
		},
	}

	// --- Test Execution ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keptTasks, evictedClusters := assessEvictionTasksForRB(tt.opt, timeNow)

			// Convert keptTasks to a slice of cluster names for easy comparison.
			keptTaskClusters := make([]string, len(keptTasks))
			for i, task := range keptTasks {
				keptTaskClusters[i] = task.FromCluster
			}

			// Sort slices before comparison to ensure test is deterministic.
			sort.Strings(keptTaskClusters)
			sort.Strings(tt.expectedKeptTasks)
			sort.Strings(evictedClusters)
			sort.Strings(tt.expectedEvictedClusters)

			require.ElementsMatch(t, tt.expectedKeptTasks, keptTaskClusters)
			require.ElementsMatch(t, tt.expectedEvictedClusters, evictedClusters)

			// Specific check for the new task timestamping scenario.
			if tt.name == "a new task should be kept and have its timestamp set" {
				if len(keptTasks) == 1 && keptTasks[0].CreationTimestamp.IsZero() {
					t.Errorf("assessEvictionTasksForRB() did not set timestamp for the new task")
				}
			}
		})
	}
}

// Test_isTaskFinishedForCRB is the test suite for isTaskFinishedForCRB.
// Its structure is parallel to Test_isTaskFinishedForRB, but for ClusterResourceBinding.
func Test_isTaskFinishedForCRB(t *testing.T) {
	// --- Test Setup ---
	timeNow := metav1.Now()
	timeout := 3 * time.Minute

	// Re-use the same mock plugin managers.
	pluginAllowMgr, _ := plugins.NewManager([]string{"mock-allow"})
	pluginDenyMgr, _ := plugins.NewManager([]string{"mock-deny"})

	// --- Test Cases ---
	tests := []struct {
		name     string
		task     *workv1alpha2.GracefulEvictionTask
		opt      assessmentOptionCRB // Use the CRB-specific option struct
		expected bool
	}{
		{
			name:     "task with SuppressDeletion=true should not be finished",
			task:     &workv1alpha2.GracefulEvictionTask{SuppressDeletion: ptr.To(true)},
			opt:      assessmentOptionCRB{binding: &workv1alpha2.ClusterResourceBinding{}, pluginMgr: pluginDenyMgr, timeout: timeout},
			expected: false,
		},
		{
			name:     "task with SuppressDeletion=false should be finished",
			task:     &workv1alpha2.GracefulEvictionTask{SuppressDeletion: ptr.To(false)},
			opt:      assessmentOptionCRB{binding: &workv1alpha2.ClusterResourceBinding{}, pluginMgr: pluginDenyMgr, timeout: timeout},
			expected: true,
		},
		{
			name:     "task exceeds timeout should be finished",
			task:     &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &metav1.Time{Time: timeNow.Add(-4 * time.Minute)}},
			opt:      assessmentOptionCRB{binding: &workv1alpha2.ClusterResourceBinding{}, pluginMgr: pluginDenyMgr, timeout: timeout},
			expected: true,
		},
		{
			name: "plugin allows but binding not scheduled yet, should not be finished",
			task: &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &timeNow},
			opt: assessmentOptionCRB{
				binding:   &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1}},
				pluginMgr: pluginAllowMgr,
				timeout:   timeout,
			},
			expected: false,
		},
		{
			name: "binding scheduled but plugin denies, should not be finished",
			task: &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &timeNow},
			opt: assessmentOptionCRB{
				binding:   &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 2}},
				pluginMgr: pluginDenyMgr,
				timeout:   timeout,
			},
			expected: false,
		},
		{
			name: "binding scheduled and plugin allows, should be finished",
			task: &workv1alpha2.GracefulEvictionTask{CreationTimestamp: &timeNow},
			opt: assessmentOptionCRB{
				binding:   &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 2}},
				pluginMgr: pluginAllowMgr,
				timeout:   timeout,
			},
			expected: true,
		},
	}

	// --- Test Execution ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the CRB-specific function under test.
			result := isTaskFinishedForCRB(context.Background(), tt.task, tt.opt)
			// Assert the result.
			if result != tt.expected {
				t.Errorf("isTaskFinishedForCRB() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test_assessEvictionTasksForCRB is the test suite for assessEvictionTasksForCRB.
// It mirrors the structure of Test_assessEvictionTasksForRB for ClusterResourceBindings.
func Test_assessEvictionTasksForCRB(t *testing.T) {
	// --- Test Setup ---
	timeNow := metav1.Now()
	defaultTimeout := 3 * time.Minute

	pluginAllowMgr, _ := plugins.NewManager([]string{"mock-allow"})
	pluginDenyMgr, _ := plugins.NewManager([]string{"mock-deny"})

	// --- Test Cases ---
	tests := []struct {
		name                    string
		opt                     assessmentOptionCRB // Use the CRB-specific option struct
		expectedKeptTasks       []string
		expectedEvictedClusters []string
	}{
		{
			name: "no tasks to assess",
			opt: assessmentOptionCRB{
				binding: &workv1alpha2.ClusterResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{},
					},
				},
				pluginMgr: pluginDenyMgr,
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{},
			expectedEvictedClusters: []string{},
		},
		{
			name: "one task timed out, another not",
			opt: assessmentOptionCRB{
				binding: &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Generation: 1},
					Status:     workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1},
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
							{FromCluster: "member1", CreationTimestamp: &metav1.Time{Time: timeNow.Add(-1 * time.Minute)}}, // Should be kept
							{FromCluster: "member2", CreationTimestamp: &metav1.Time{Time: timeNow.Add(-5 * time.Minute)}}, // Should be evicted
						},
					},
				},
				pluginMgr: pluginDenyMgr,
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{"member1"},
			expectedEvictedClusters: []string{"member2"},
		},
		{
			name: "a task is evicted early by a plugin",
			opt: assessmentOptionCRB{
				binding: &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Generation: 1},
					Status:     workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1}, // hasScheduled is true
					Spec: workv1alpha2.ResourceBindingSpec{
						GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
							{FromCluster: "member1", CreationTimestamp: &timeNow}, // Not timed out, but plugin will allow
						},
					},
				},
				pluginMgr: pluginAllowMgr, // Using the manager that allows eviction
				timeout:   defaultTimeout,
			},
			expectedKeptTasks:       []string{},
			expectedEvictedClusters: []string{"member1"},
		},
	}

	// --- Test Execution ---
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keptTasks, evictedClusters := assessEvictionTasksForCRB(tt.opt, timeNow)

			// Convert keptTasks to a slice of cluster names for easy comparison.
			keptTaskClusters := make([]string, len(keptTasks))
			for i, task := range keptTasks {
				keptTaskClusters[i] = task.FromCluster
			}

			// Sort slices before comparison to ensure test is deterministic.
			sort.Strings(keptTaskClusters)
			sort.Strings(tt.expectedKeptTasks)
			sort.Strings(evictedClusters)
			sort.Strings(tt.expectedEvictedClusters)

			assert.Equal(t, tt.expectedKeptTasks, keptTaskClusters)
			assert.Equal(t, tt.expectedEvictedClusters, evictedClusters)
		})
	}
}

// Test_nextRetry remains unchanged as it is a generic utility function.
func Test_nextRetry(t *testing.T) {
	timeNow := metav1.Now()
	timeout := time.Minute * 20
	type args struct {
		tasks           []workv1alpha2.GracefulEvictionTask
		gracefulTimeout time.Duration
		timeNow         time.Time
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "no tasks",
			args: args{
				tasks:           []workv1alpha2.GracefulEvictionTask{},
				gracefulTimeout: timeout,
				timeNow:         timeNow.Time,
			},
			want: 0,
		},
		{
			name: "find the minimum retry interval",
			args: args{
				tasks: []workv1alpha2.GracefulEvictionTask{
					{CreationTimestamp: &metav1.Time{Time: timeNow.Add(-19 * time.Minute)}},
					{CreationTimestamp: &metav1.Time{Time: timeNow.Add(-10 * time.Minute)}},
				},
				gracefulTimeout: timeout,
				timeNow:         timeNow.Time,
			},
			want: 1 * time.Minute,
		},
		{
			name: "only suppression tasks should result in no retry",
			args: args{
				tasks: []workv1alpha2.GracefulEvictionTask{
					{SuppressDeletion: ptr.To(true)},
					{SuppressDeletion: ptr.To(true)},
				},
				gracefulTimeout: timeout,
				timeNow:         timeNow.Time,
			},
			want: 0,
		},
		{
			name: "task with custom grace period",
			args: args{
				tasks: []workv1alpha2.GracefulEvictionTask{
					{
						CreationTimestamp:  &metav1.Time{Time: timeNow.Add(-5 * time.Minute)},
						GracePeriodSeconds: ptr.To[int32](600), // 10 minutes
					},
				},
				gracefulTimeout: timeout, // default timeout is ignored
				timeNow:         timeNow.Time,
			},
			want: 5 * time.Minute, // 10 minutes (grace period) - 5 minutes (elapsed)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextRetry(tt.args.tasks, tt.args.gracefulTimeout, tt.args.timeNow); got != tt.want {
				t.Errorf("nextRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
