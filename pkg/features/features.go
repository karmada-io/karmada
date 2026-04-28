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

package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
	logsv1 "k8s.io/component-base/logs/api/v1"
)

const (
	// Failover controls whether the scheduler should reschedule
	// workloads on cluster failure.
	// When enabled, Karmada will automatically migrate workloads
	// from a failed cluster to other available clusters.
	//
	// Note: This feature does not control application failover,
	// which is managed separately via the PropagationPolicy or
	// ClusterPropagationPolicy.
	Failover featuregate.Feature = "Failover"

	// GracefulEviction controls whether to perform graceful evictions
	// during both cluster failover and application failover.
	// When used for cluster failover, it takes effect only when the
	// Failover feature is enabled.
	// Graceful eviction ensures that workloads are migrated in a
	// controlled manner, minimizing disruption to applications.
	GracefulEviction featuregate.Feature = "GracefulEviction"

	// PropagateDeps indicates if relevant resources should be propagated automatically
	PropagateDeps featuregate.Feature = "PropagateDeps"

	// CustomizedClusterResourceModeling indicates if enable cluster resource custom modeling.
	CustomizedClusterResourceModeling featuregate.Feature = "CustomizedClusterResourceModeling"

	// PolicyPreemption indicates if high-priority PropagationPolicy/ClusterPropagationPolicy could preempt resource templates which are matched by low-priority PropagationPolicy/ClusterPropagationPolicy.
	PolicyPreemption featuregate.Feature = "PropagationPolicyPreemption"

	// MultiClusterService indicates if enable multi-cluster service function.
	MultiClusterService featuregate.Feature = "MultiClusterService"

	// ResourceQuotaEstimate indicates if enable resource quota check in estimator
	ResourceQuotaEstimate featuregate.Feature = "ResourceQuotaEstimate"

	// StatefulFailoverInjection controls whether Karmada collects state information
	// from the source cluster during a failover event for stateful applications and
	// injects this information into the application configuration when it is moved
	// to the target cluster.
	//
	// owner: @mszacillo, @XiShanYongYe-Chang
	// alpha: v1.12
	StatefulFailoverInjection featuregate.Feature = "StatefulFailoverInjection"

	// PriorityBasedScheduling controls whether the Priority-Based Scheduling feature is enabled.
	// When enabled, the scheduler prioritizes applications with higher priority values, ensuring they
	// are scheduled before lower-priority ones. This is useful for cluster environments where critical
	// workloads must be processed first to meet SLA requirements.
	//
	// owner: @LeonZh0u, @seanlaii, @wengyao04, @whitewindmills, @zclyne
	// alpha: v1.13
	PriorityBasedScheduling featuregate.Feature = "PriorityBasedScheduling"

	// FederatedQuotaEnforcement controls whether the existing FederatedResourceQuota enhancement feature is enabled.
	// When enabled, the existing FederatedResourceQuota can impose namespaced resource limits directly on the Karmada control-plane level
	//
	// owner: @mszacillo, @RainbowMango, @zhzhuang-zju, @seanlaii, @liwang0513
	// alpha: v1.14
	FederatedQuotaEnforcement featuregate.Feature = "FederatedQuotaEnforcement"

	// LoggingAlphaOptions allows fine-tuning of experimental, alpha-quality logging options.
	// Inherited from Kubernetes. Ref: https://github.com/kubernetes/component-base/blob/release-1.32/logs/api/v1/kube_features.go#L45
	//
	// owner: @jabellard
	// alpha: v1.15
	LoggingAlphaOptions = logsv1.LoggingAlphaOptions

	// LoggingBetaOptions allows fine-tuning of experimental, beta-quality logging options.
	// Inherited from Kubernetes. Ref: https://github.com/kubernetes/component-base/blob/release-1.32/logs/api/v1/kube_features.go#L54
	//
	// owner: @jabellard
	// beta: v1.15
	LoggingBetaOptions = logsv1.LoggingBetaOptions

	// ContextualLogging enables looking up a logger from a context.Context instead of using
	// the global fallback logger and manipulating the logger that is
	// used by a call chain.
	// Inherited from Kubernetes. Ref: https://github.com/kubernetes/component-base/blob/release-1.32/logs/api/v1/kube_features.go#L32
	//
	// owner: @jabellard
	// beta: v1.15
	ContextualLogging = logsv1.ContextualLogging

	// MultiplePodTemplatesScheduling enables enhanced, resource-aware scheduling for workloads with multiple pod templates.
	// When enabled, the scheduler and resource interpreter will use the new 'GetComponents' hook and 'components' field
	// to support accurate resource estimation and scheduling for complex CRDs (e.g., FlinkDeployments, RayJob, VolcanoJob) that consist of
	// multiple components with different resource requirements. This allows for more precise FederatedResourceQuota
	// calculations and better placement decisions.
	//
	// owner: @mszacillo, @Dyex719, @RainbowMango, @XiShanYongYe-Chang, @zhzhuang-zju, @seanlaii
	// alpha: v1.15
	MultiplePodTemplatesScheduling featuregate.Feature = "MultiplePodTemplatesScheduling"

	// ControllerPriorityQueue controls whether the controller-runtime Priority Queue for controllers is enabled.
	// When enabled, during the startup phase of karmada-controller-manager and karmada-agent,
	// controllers implemented with controller-runtime will prioritize processing recent cluster changes first,
	// while deferring items without recent updates that still need to be processed to the end of the queue.
	// This helps the system quickly catch up with the latest state and reduces the perceived
	// backlog from external users.
	// However, enabling this feature may increase the overall number of reconciliations. This is
	// because newer events spend less time in the queue, which reduces the chance of them being
	// deduplicated before processing.
	//
	// owner: @zach593
	// alpha: v1.15
	// beta: v1.17
	ControllerPriorityQueue featuregate.Feature = "ControllerPriorityQueue"

	// WorkloadAffinity enables the scheduler to group or separate workloads across clusters
	// based on their defined workload affinity or anti-affinity groups, which are managed
	// via the PropagationPolicy or ClusterPropagationPolicy.
	// When enabled, the scheduler maintains a global in-memory index of affinity groups.
	// Based on a workload's affinity group, the scheduler filters out candidate clusters
	// that would violate the workload's affinity settings.
	//
	// owner: @mszacillo, @RainbowMango, @kevin-wangzefeng
	// alpha: v1.17
	WorkloadAffinity featuregate.Feature = "WorkloadAffinity"
)

var (
	// FeatureGate is a shared global FeatureGate.
	FeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	// DefaultFeatureGates is the default feature gates of Karmada.
	DefaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		// Failover(cluster failover) is disabled by default because it involves migrating
		// all resources in the cluster, which can have significant impacts, it should be
		// explicitly enabled by administrators after fully evaluation to avoid unexpected
		// incidents.
		Failover:                          {Default: false, PreRelease: featuregate.Beta},
		GracefulEviction:                  {Default: true, PreRelease: featuregate.Beta},
		PropagateDeps:                     {Default: true, PreRelease: featuregate.Beta},
		CustomizedClusterResourceModeling: {Default: true, PreRelease: featuregate.Beta},
		PolicyPreemption:                  {Default: false, PreRelease: featuregate.Alpha},
		MultiClusterService:               {Default: false, PreRelease: featuregate.Alpha},
		ResourceQuotaEstimate:             {Default: false, PreRelease: featuregate.Alpha},
		StatefulFailoverInjection:         {Default: false, PreRelease: featuregate.Alpha},
		PriorityBasedScheduling:           {Default: false, PreRelease: featuregate.Alpha},
		FederatedQuotaEnforcement:         {Default: false, PreRelease: featuregate.Alpha},
		LoggingAlphaOptions:               {Default: false, PreRelease: featuregate.Alpha},
		LoggingBetaOptions:                {Default: true, PreRelease: featuregate.Beta},
		ContextualLogging:                 {Default: true, PreRelease: featuregate.Beta},
		MultiplePodTemplatesScheduling:    {Default: false, PreRelease: featuregate.Alpha},
		ControllerPriorityQueue:           {Default: true, PreRelease: featuregate.Beta},
		WorkloadAffinity:                  {Default: false, PreRelease: featuregate.Alpha},
	}
)

func init() {
	runtime.Must(FeatureGate.Add(DefaultFeatureGates))
}
