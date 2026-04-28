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

package framework

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	// PollInterval defines the interval time for a poll operation.
	PollInterval = 5 * time.Second
	// PollTimeout defines the time after which the poll operation times out.
	PollTimeout = 420 * time.Second
	// metricsCreationDelay defines the maximum time metrics not yet available for pod.
	metricsCreationDelay = 2 * time.Minute
)

const (
	// TaintClusterUnscheduler will be added when cluster becomes unschedulable
	// and removed when cluster becomes schedulable.
	TaintClusterUnscheduler = "unschedulable"
	// TaintClusterNotReady will be added when cluster is not ready
	// and removed when cluster becomes ready.
	TaintClusterNotReady = "not-ready"
	// TaintClusterUnreachable will be added when cluster becomes unreachable
	// (corresponding to ClusterConditionReady status ConditionUnknown)
	// and removed when cluster becomes reachable (ClusterConditionReady status ConditionTrue).
	TaintClusterUnreachable = "unreachable"
	// TaintClusterTerminating will be added when cluster is terminating.
	TaintClusterTerminating = "terminating"
)

var (
	// UnreachableTaintTemplate is the taint for when a cluster becomes unreachable.
	// Used for taint based eviction.
	UnreachableTaintTemplate = &corev1.Taint{
		Key:    TaintClusterUnreachable,
		Effect: corev1.TaintEffectNoExecute,
	}

	// UnreachableTaintTemplateForSched is the taint for when a cluster becomes unreachable.
	// Used for taint based schedule.
	UnreachableTaintTemplateForSched = &corev1.Taint{
		Key:    TaintClusterUnreachable,
		Effect: corev1.TaintEffectNoSchedule,
	}

	// NotReadyTaintTemplate is the taint for when a cluster is not ready for executing resources.
	// Used for taint based eviction.
	NotReadyTaintTemplate = &corev1.Taint{
		Key:    TaintClusterNotReady,
		Effect: corev1.TaintEffectNoExecute,
	}

	// NotReadyTaintTemplateForSched is the taint for when a cluster is not ready for executing resources.
	// Used for taint based schedule.
	NotReadyTaintTemplateForSched = &corev1.Taint{
		Key:    TaintClusterNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}

	// TerminatingTaintTemplate is the taint for when a cluster is terminating executing resources.
	// Used for taint based eviction.
	TerminatingTaintTemplate = &corev1.Taint{
		Key:    TaintClusterTerminating,
		Effect: corev1.TaintEffectNoExecute,
	}
)
