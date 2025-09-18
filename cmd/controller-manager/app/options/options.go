/*
Copyright 2020 The Karmada Authors.

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

package options

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/karmada-io/karmada/pkg/controllers/federatedhpa/config"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	// Controllers is the list of controllers to enable or disable
	// '*' means "all enabled by default controllers"
	// 'foo' means "enable 'foo'"
	// '-foo' means "disable 'foo'"
	// first item for a particular name wins
	Controllers []string
	// LeaderElection defines the configuration of leader election client.
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	// ClusterStatusUpdateFrequency is the frequency that controller computes and report cluster status.
	// It must work with ClusterMonitorGracePeriod(--cluster-monitor-grace-period) in karmada-controller-manager.
	ClusterStatusUpdateFrequency metav1.Duration
	// ClusterLeaseDuration is a duration that candidates for a lease need to wait to force acquire it.
	// This is measure against time of last observed lease RenewTime.
	ClusterLeaseDuration metav1.Duration
	// ClusterLeaseRenewIntervalFraction is a fraction coordinated with ClusterLeaseDuration that
	// how long the current holder of a lease has last updated the lease.
	ClusterLeaseRenewIntervalFraction float64
	// ClusterSuccessThreshold is the duration of successes for the cluster to be considered healthy after recovery.
	ClusterSuccessThreshold metav1.Duration
	// ClusterFailureThreshold is the duration of failure for the cluster to be considered unhealthy.
	ClusterFailureThreshold metav1.Duration
	// ClusterMonitorPeriod represents cluster-controller monitoring period, i.e. how often does
	// cluster-controller check cluster health signal posted from cluster-status-controller.
	// This value should be lower than ClusterMonitorGracePeriod.
	ClusterMonitorPeriod metav1.Duration
	// ClusterMonitorGracePeriod represents the grace period after last cluster health probe time.
	// If it doesn't receive update for this amount of time, it will start posting
	// "ClusterReady==ConditionUnknown".
	ClusterMonitorGracePeriod metav1.Duration
	// When cluster is just created, e.g. agent bootstrap or cluster join, we give a longer grace period.
	ClusterStartupGracePeriod metav1.Duration
	// SkippedPropagatingAPIs indicates comma separated resources that should be skipped for propagating.
	SkippedPropagatingAPIs string
	// SkippedPropagatingNamespaces is a list of namespaces that will be skipped for propagating.
	SkippedPropagatingNamespaces []string
	// ClusterAPIContext is the name of the cluster context in cluster-api management cluster KUBECONFIG file.
	// Default value is the current-context.
	ClusterAPIContext string
	// ClusterAPIKubeconfig holds the cluster-api management cluster KUBECONFIG file path.
	ClusterAPIKubeconfig string
	// ClusterAPIQPS is the QPS to use while talking with cluster kube-apiserver.
	ClusterAPIQPS float32
	// ClusterAPIBurst is the burst to allow while talking with cluster kube-apiserver.
	ClusterAPIBurst int
	// KubeAPIQPS is the QPS to use while talking with karmada-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int
	// ClusterCacheSyncTimeout is the timeout period waiting for cluster cache to sync
	ClusterCacheSyncTimeout metav1.Duration
	// ResyncPeriod is the base frequency the informers are resynced.
	// Defaults to 0, which means the created informer will never do resyncs.
	ResyncPeriod metav1.Duration
	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string
	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	// It can be set to "0" to disable serving the health probe.
	// Defaults to ":10357".
	HealthProbeBindAddress string
	// ConcurrentClusterSyncs is the number of cluster objects that are
	// allowed to sync concurrently.
	ConcurrentClusterSyncs int
	// ConcurrentClusterResourceBindingSyncs is the number of clusterresourcebinding objects that are
	// allowed to sync concurrently.
	ConcurrentClusterResourceBindingSyncs int
	// ConcurrentWorkSyncs is the number of Work objects that are
	// allowed to sync concurrently.
	ConcurrentWorkSyncs int
	// ConcurrentResourceBindingSyncs is the number of resourcebinding objects that are
	// allowed to sync concurrently.
	ConcurrentResourceBindingSyncs int
	// ConcurrentNamespaceSyncs is the number of Namespace objects that are
	// allowed to sync concurrently.
	ConcurrentNamespaceSyncs int
	// ConcurrentPropagationPolicySyncs is the number of PropagationPolicy that are allowed to sync concurrently.
	ConcurrentPropagationPolicySyncs int
	// ConcurrentClusterPropagationPolicySyncs is the number of ClusterPropagationPolicy that are allowed to sync concurrently.
	ConcurrentClusterPropagationPolicySyncs int
	// ConcurrentResourceTemplateSyncs is the number of resource templates that are allowed to sync concurrently.
	ConcurrentResourceTemplateSyncs int
	// ConcurrentDependentResourceSyncs is the number of dependent resource that are allowed to sync concurrently.
	ConcurrentDependentResourceSyncs int
	// If set to true enables NoExecute Taints and will evict all not-tolerating
	// objects propagating on Clusters tainted with this kind of Taints.
	EnableTaintManager bool
	// GracefulEvictionTimeout is the timeout period waiting for the grace-eviction-controller performs the final
	// removal since the workload(resource) has been moved to the graceful eviction tasks.
	GracefulEvictionTimeout metav1.Duration

	RateLimiterOpts            ratelimiterflag.Options
	ProfileOpts                profileflag.Options
	HPAControllerConfiguration config.HPAControllerConfiguration
	// EnableClusterResourceModeling indicates if enable cluster resource modeling.
	// The resource modeling might be used by the scheduler to make scheduling decisions
	// in scenario of dynamic replica assignment based on cluster free resources.
	// Disable if it does not fit your cases for better performance.
	EnableClusterResourceModeling bool
	// FederatedResourceQuotaOptions holds configurations for FederatedResourceQuota reconciliation.
	FederatedResourceQuotaOptions FederatedResourceQuotaOptions
	// FailoverOptions holds the Failover configurations.
	FailoverOptions FailoverOptions
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: names.NamespaceKarmadaSystem,
			ResourceName:      names.KarmadaControllerManagerComponentName,
		},
	}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet, allControllers, disabledByDefaultControllers []string) {
	flags.StringSliceVar(&o.Controllers, "controllers", []string{"*"}, fmt.Sprintf(
		"A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller named 'foo', '-foo' disables the controller named 'foo'. \nAll controllers: %s.\nDisabled-by-default controllers: %s",
		strings.Join(allControllers, ", "), strings.Join(disabledByDefaultControllers, ", "),
	))
	flags.DurationVar(&o.ClusterStatusUpdateFrequency.Duration, "cluster-status-update-frequency", 10*time.Second,
		"Specifies how often karmada-controller-manager posts cluster status to karmada-apiserver.")
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", names.NamespaceKarmadaSystem, "The namespace of resource object that is used for locking during leader election.")
	flags.DurationVar(&o.LeaderElection.LeaseDuration.Duration, "leader-elect-lease-duration", defaultElectionLeaseDuration.Duration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	flags.DurationVar(&o.LeaderElection.RenewDeadline.Duration, "leader-elect-renew-deadline", defaultElectionRenewDeadline.Duration, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	flags.DurationVar(&o.LeaderElection.RetryPeriod.Duration, "leader-elect-retry-period", defaultElectionRetryPeriod.Duration, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	flags.DurationVar(&o.ClusterLeaseDuration.Duration, "cluster-lease-duration", 40*time.Second,
		"Specifies the expiration period of a cluster lease.")
	flags.Float64Var(&o.ClusterLeaseRenewIntervalFraction, "cluster-lease-renew-interval-fraction", 0.25,
		"Specifies the cluster lease renew interval fraction.")
	flags.DurationVar(&o.ClusterSuccessThreshold.Duration, "cluster-success-threshold", 30*time.Second, "The duration of successes for the cluster to be considered healthy after recovery.")
	flags.DurationVar(&o.ClusterFailureThreshold.Duration, "cluster-failure-threshold", 30*time.Second, "The duration of failure for the cluster to be considered unhealthy.")
	flags.DurationVar(&o.ClusterMonitorPeriod.Duration, "cluster-monitor-period", 5*time.Second,
		"Specifies how often karmada-controller-manager monitors cluster health status.")
	flags.DurationVar(&o.ClusterMonitorGracePeriod.Duration, "cluster-monitor-grace-period", 40*time.Second,
		"Specifies the grace period of allowing a running cluster to be unresponsive before marking it unhealthy.")
	flags.DurationVar(&o.ClusterStartupGracePeriod.Duration, "cluster-startup-grace-period", 60*time.Second,
		"Specifies the grace period of allowing a cluster to be unresponsive during startup before marking it unhealthy.")
	flags.StringVar(&o.SkippedPropagatingAPIs, "skipped-propagating-apis", "", "Semicolon separated resources that should be skipped from propagating in addition to the default skip list(cluster.karmada.io;policy.karmada.io;work.karmada.io). Supported formats are:\n"+
		"<group> for skip resources with a specific API group(e.g. networking.k8s.io),\n"+
		"<group>/<version> for skip resources with a specific API version(e.g. networking.k8s.io/v1beta1),\n"+
		"<group>/<version>/<kind>,<kind> for skip one or more specific resource(e.g. networking.k8s.io/v1beta1/Ingress,IngressClass) where the kinds are case-insensitive.")
	flags.StringSliceVar(&o.SkippedPropagatingNamespaces, "skipped-propagating-namespaces", []string{"kube-.*"},
		"Comma-separated namespaces that should be skipped from propagating.\n"+
			"Note: 'karmada-system', 'karmada-cluster' and 'karmada-es-.*' are Karmada reserved namespaces that will always be skipped.")
	flags.StringVar(&o.ClusterAPIContext, "cluster-api-context", "", "Name of the cluster context in cluster-api management cluster kubeconfig file.")
	flags.StringVar(&o.ClusterAPIKubeconfig, "cluster-api-kubeconfig", "", "Path to the cluster-api management cluster kubeconfig file.")
	flags.Float32Var(&o.ClusterAPIQPS, "cluster-api-qps", 40.0, "QPS to use while talking with cluster kube-apiserver.")
	flags.IntVar(&o.ClusterAPIBurst, "cluster-api-burst", 60, "Burst to use while talking with cluster kube-apiserver.")
	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver.")
	flags.DurationVar(&o.ClusterCacheSyncTimeout.Duration, "cluster-cache-sync-timeout", util.CacheSyncTimeout, "Timeout period waiting for cluster cache to sync.")
	flags.DurationVar(&o.ResyncPeriod.Duration, "resync-period", 0, "Base frequency the informers are resynced.")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving.")
	flags.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", ":10357", "The TCP address that the controller should bind to for serving health probes(e.g. 127.0.0.1:10357, :10357). It can be set to \"0\" to disable serving the health probe. Defaults to 0.0.0.0:10357.")
	flags.IntVar(&o.ConcurrentClusterSyncs, "concurrent-cluster-syncs", 5, "The number of Clusters that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentClusterResourceBindingSyncs, "concurrent-clusterresourcebinding-syncs", 5, "The number of ClusterResourceBindings that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentResourceBindingSyncs, "concurrent-resourcebinding-syncs", 5, "The number of ResourceBindings that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentWorkSyncs, "concurrent-work-syncs", 5, "The number of Works that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentNamespaceSyncs, "concurrent-namespace-syncs", 1, "The number of Namespaces that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentPropagationPolicySyncs, "concurrent-propagation-policy-syncs", 1, "The number of PropagationPolicy that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentClusterPropagationPolicySyncs, "concurrent-cluster-propagation-policy-syncs", 1, "The number of ClusterPropagationPolicy that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentResourceTemplateSyncs, "concurrent-resource-template-syncs", 5, "The number of resource templates that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentDependentResourceSyncs, "concurrent-dependent-resource-syncs", 2, "The number of dependent resource that are allowed to sync concurrently.")
	flags.BoolVar(&o.EnableTaintManager, "enable-taint-manager", true, "If set to true enables NoExecute Taints and will evict all not-tolerating objects propagating on Clusters tainted with this kind of Taints.")
	flags.DurationVar(&o.GracefulEvictionTimeout.Duration, "graceful-eviction-timeout", 10*time.Minute, "Specifies the timeout period waiting for the graceful-eviction-controller performs the final removal since the workload(resource) has been moved to the graceful eviction tasks.")
	flags.BoolVar(&o.EnableClusterResourceModeling, "enable-cluster-resource-modeling", true, "Enable means controller would build resource modeling for each cluster by syncing Nodes and Pods resources.\n"+
		"The resource modeling might be used by the scheduler to make scheduling decisions in scenario of dynamic replica assignment based on cluster free resources.\n"+
		"Disable if it does not fit your cases for better performance.")

	o.RateLimiterOpts.AddFlags(flags)
	o.ProfileOpts.AddFlags(flags)
	o.HPAControllerConfiguration.AddFlags(flags)
	o.FederatedResourceQuotaOptions.AddFlags(flags)
	o.FailoverOptions.AddFlags(flags)
	features.FeatureGate.AddFlag(flags)
}

// SkippedNamespacesRegexps precompiled regular expressions
func (o *Options) SkippedNamespacesRegexps() []*regexp.Regexp {
	skippedPropagatingNamespaces := make([]*regexp.Regexp, len(o.SkippedPropagatingNamespaces))
	for index, ns := range o.SkippedPropagatingNamespaces {
		skippedPropagatingNamespaces[index] = regexp.MustCompile(fmt.Sprintf("^%s$", ns))
	}
	return skippedPropagatingNamespaces
}
