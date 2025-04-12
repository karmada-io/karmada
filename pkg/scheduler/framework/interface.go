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

// Given mockgen does not group imports according to our project's conventions.
// Following the mockgen command, we run 'goimports', which reformats the
// generated file to ensure that all imports are properly grouped and sorted,
// maintaining consistency with the rest of our codebase.
//go:generate mockgen -source=interface.go -destination=testing/mock_interface.go -package=testing FilterPlugin ScorePlugin ScoreExtensions
//go:generate goimports -local "github.com/karmada-io/karmada" -w testing/mock_interface.go

package framework

import (
	"context"
	"errors"
	"strings"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

const (
	// MinClusterScore is the minimum score a Score plugin is expected to return.
	MinClusterScore int64 = 0

	// MaxClusterScore is the maximum score a Score plugin is expected to return.
	MaxClusterScore int64 = 100
)

// Framework manages the set of plugins in use by the scheduling framework.
// Configured plugins are called at specified points in a scheduling context.
type Framework interface {

	// RunFilterPlugins runs the set of configured Filter plugins for resources on
	// the given cluster.
	RunFilterPlugins(ctx context.Context, bindingSpec *workv1alpha2.ResourceBindingSpec, bindingStatus *workv1alpha2.ResourceBindingStatus, cluster *clusterv1alpha1.Cluster) *Result

	// RunScorePlugins runs the set of configured Score plugins, it returns a map of plugin names to scores
	RunScorePlugins(ctx context.Context, spec *workv1alpha2.ResourceBindingSpec, clusters []*clusterv1alpha1.Cluster) (PluginToClusterScores, *Result)
}

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

// FilterPlugin is an interface for filter plugins. These filters are used to filter out clusters
// that are not fit for the resource.
type FilterPlugin interface {
	Plugin
	// Filter is called by the scheduling framework.
	Filter(ctx context.Context, bindingSpec *workv1alpha2.ResourceBindingSpec, bindingStatus *workv1alpha2.ResourceBindingStatus, cluster *clusterv1alpha1.Cluster) *Result
}

// Result indicates the result of running a plugin. It consists of a code, a
// message and (optionally) an error. When the status code is not `Success`,
// the reasons should explain why.
type Result struct {
	code    Code
	reasons []string
	err     error
}

// Code is the Status code/type which is returned from plugins.
type Code int

// These are predefined codes used in a Status.
const (
	// Success means that plugin ran correctly and found resource schedulable.
	// NOTE: A nil status is also considered as "Success".
	Success Code = iota
	// Unschedulable is used when a plugin finds the resource unschedulable.
	// The accompanying status message should explain why it is unschedulable.
	Unschedulable
	// Error is used for internal plugin errors, unexpected input, etc.
	Error
)

// This list should be exactly the same as the codes iota defined above in the same order.
var codes = []string{"Success", "Unschedulable", "Error"}

func (c Code) String() string {
	return codes[c]
}

// NewResult makes a result out of the given arguments and returns its pointer.
func NewResult(code Code, reasons ...string) *Result {
	s := &Result{
		code:    code,
		reasons: reasons,
	}
	if code == Error {
		s.err = errors.New(strings.Join(reasons, ","))
	}
	return s
}

// PluginToResult maps plugin name to Result.
type PluginToResult map[string]*Result

// Merge merges the statuses in the map into one. The resulting status code have the following
// precedence: Error, Unschedulable.
func (p PluginToResult) Merge() *Result {
	if len(p) == 0 {
		return nil
	}

	finalStatus := NewResult(Success)
	var hasUnschedulable bool
	for _, s := range p {
		switch s.code {
		case Error:
			finalStatus.err = s.err
		case Unschedulable:
			hasUnschedulable = true
		}
		finalStatus.code = s.code
		finalStatus.reasons = append(finalStatus.reasons, s.reasons...)
	}

	if finalStatus.err != nil {
		finalStatus.code = Error
	} else if hasUnschedulable {
		finalStatus.code = Unschedulable
	}
	return finalStatus
}

// IsSuccess returns true if and only if "Result" is nil or Code is "Success".
func (s *Result) IsSuccess() bool {
	return s == nil || s.code == Success
}

// AsError returns nil if the Result is a success; otherwise returns an "error" object
// with a concatenated message on reasons of the Result.
func (s *Result) AsError() error {
	if s.IsSuccess() {
		return nil
	}
	if s.err != nil {
		return s.err
	}
	return errors.New(strings.Join(s.reasons, ", "))
}

// Reasons returns reasons of the Result.
func (s *Result) Reasons() []string {
	return s.reasons
}

// Code returns code of the Result.
func (s *Result) Code() Code {
	if s == nil {
		return Success
	}
	return s.code
}

// AsResult wraps an error in a Result.
func AsResult(err error) *Result {
	return &Result{
		code:    Error,
		reasons: []string{err.Error()},
		err:     err,
	}
}

// ScorePlugin is an interface that must be implemented by "Score" plugins to rank
// clusters that passed the filtering phase.
type ScorePlugin interface {
	Plugin
	// Score is called on each filtered cluster. It must return success and an integer
	// indicating the rank of the cluster. All scoring plugins must return success or
	// the resource will be rejected.
	Score(ctx context.Context, spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *Result)

	// ScoreExtensions returns a ScoreExtensions interface
	// if it implements one, or nil if does not.
	ScoreExtensions() ScoreExtensions
}

// ScoreExtensions is an interface for Score extended functionality.
type ScoreExtensions interface {
	// NormalizeScore is called for all cluster scores produced
	// by the same plugin's "Score"
	NormalizeScore(ctx context.Context, scores ClusterScoreList) *Result
}

// ClusterScore represent the cluster score.
type ClusterScore struct {
	Cluster *clusterv1alpha1.Cluster
	Score   int64
}

// ClusterScoreList declares a list of clusters and their scores.
type ClusterScoreList []ClusterScore

// PluginToClusterScores declares a map from plugin name to its ClusterScoreList.
type PluginToClusterScores map[string]ClusterScoreList
