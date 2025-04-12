/*
Copyright 2024 The Karmada Authors.

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

package framework

import (
	"context"
	"errors"
	"strings"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
)

// Framework manages the set of plugins in use by the estimator.
type Framework interface {
	Handle
	// RunEstimateReplicasPlugins runs the set of configured EstimateReplicasPlugins
	// for estimating replicas based on the given replicaRequirements.
	// It returns an integer and an Result.
	// The integer represents the minimum calculated value of estimated replicas from each EstimateReplicasPlugin.
	// The Result contains code, reasons and error
	// it is merged from all plugins returned result codes
	RunEstimateReplicasPlugins(ctx context.Context, snapshot *schedcache.Snapshot, replicaRequirements *pb.ReplicaRequirements) (int32, *Result)
	// TODO(wengyao04): we can add filter and score plugin extension points if needed in the future
}

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

// EstimateReplicasPlugin is an interface for replica estimation plugins.
// These estimators are used to estimate the replicas for a given pb.ReplicaRequirements
type EstimateReplicasPlugin interface {
	Plugin
	// Estimate is called for each MaxAvailableReplicas request.
	// It returns an integer and an error
	// The integer representing the number of calculated replica for the given replicaRequirements
	// The Result contains code, reasons and error
	// it is merged from all plugins returned result codes
	Estimate(ctx context.Context, snapshot *schedcache.Snapshot, replicaRequirements *pb.ReplicaRequirements) (int32, *Result)
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization. Plugins
// must store and use this handle to call framework functions.
// We follow the design pattern as kubernetes scheduler framework
type Handle interface {
	ClientSet() clientset.Interface
	SharedInformerFactory() informers.SharedInformerFactory
}

// Code is the Status code/type which is returned from plugins.
type Code int

// Result indicates the result of running a plugin. It consists of a code, a
// message and (optionally) an error. When the status code is not `Success`,
// the reasons should explain why.
type Result struct {
	code    Code
	reasons []string
	err     error
}

// These are predefined codes used in a Status.
const (
	// Success means that plugin ran correctly and found resource schedulable.
	// NOTE: A nil status is also considered as "Success".
	Success Code = iota
	// Unschedulable is used when a plugin finds the resource unschedulable.
	// The accompanying status message should explain why the it is unschedulable.
	Unschedulable
	// Nooperation is used when a plugin is disabled or the plugin list are empty
	Noopperation
	// Error is used for internal plugin errors, unexpected input, etc.
	Error
)

// This list should be exactly the same as the codes iota defined above in the same order.
var codes = []string{"Success", "Unschedulable", "Nooperation", "Error"}

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
// precedence: Error, Unschedulable, Disabled.
func (p PluginToResult) Merge() *Result {
	if len(p) == 0 {
		return NewResult(Noopperation, "plugin results are empty")
	}

	finalStatus := NewResult(Success)
	var hasUnschedulable bool
	hasAllNoOp := true
	for _, s := range p {
		switch s.code {
		case Error:
			finalStatus.err = s.err
		case Unschedulable:
			hasUnschedulable = true
		}
		if s.code != Noopperation {
			hasAllNoOp = false
		}
		finalStatus.code = s.code
		finalStatus.reasons = append(finalStatus.reasons, s.reasons...)
	}

	if finalStatus.err != nil {
		finalStatus.code = Error
	} else if hasUnschedulable {
		finalStatus.code = Unschedulable
	} else if hasAllNoOp {
		finalStatus.code = Noopperation
	} else {
		finalStatus.code = Success
	}
	return finalStatus
}

// IsSuccess returns true if and only if "Result" is nil or Code is "Success".
func (s *Result) IsSuccess() bool {
	return s == nil || s.code == Success
}

// IsUnschedulable returns true if "Result" is not nil and Code is "Unschedulable".
func (s *Result) IsUnschedulable() bool {
	return s != nil && s.code == Unschedulable
}

// IsNoOperation returns true if "Result" is not nil and Code is "Nooperation"
// ToDo (wengyao04): we can remove it once we include node resource estimation as the default plugin in the future
func (s *Result) IsNoOperation() bool {
	return s != nil && s.code == Noopperation
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
