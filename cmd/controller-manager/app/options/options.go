package options

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	defaultBindAddress = "0.0.0.0"
	defaultPort        = 10357
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
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the the server serves at.
	// Note: We hope support https in the future once controller-runtime provides the functionality.
	SecurePort int
	// ClusterStatusUpdateFrequency is the frequency that controller computes and report cluster status.
	// It must work with ClusterMonitorGracePeriod(--cluster-monitor-grace-period) in karmada-controller-manager.
	ClusterStatusUpdateFrequency metav1.Duration
	// FailoverEvictionTimeout is the grace period for deleting scheduling result on failed clusters.
	FailoverEvictionTimeout metav1.Duration
	// ClusterLeaseDuration is a duration that candidates for a lease need to wait to force acquire it.
	// This is measure against time of last observed lease RenewTime.
	ClusterLeaseDuration metav1.Duration
	// ClusterLeaseRenewIntervalFraction is a fraction coordinated with ClusterLeaseDuration that
	// how long the current holder of a lease has last updated the lease.
	ClusterLeaseRenewIntervalFraction float64
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
	// concurrentClusterSyncs is the number of cluster objects that are
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
	// ConcurrentResourceTemplateSyncs is the number of resource templates that are allowed to sync concurrently.
	ConcurrentResourceTemplateSyncs int

	RateLimiterOpts ratelimiterflag.Options
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: util.NamespaceKarmadaSystem,
			ResourceName:      "karmada-controller-manager",
		},
	}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet, allControllers, disabledByDefaultControllers []string) {
	flags.StringSliceVar(&o.Controllers, "controllers", []string{"*"}, fmt.Sprintf(
		"A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller named 'foo', '-foo' disables the controller named 'foo'. \nAll controllers: %s.\nDisabled-by-default controllers: %s",
		strings.Join(allControllers, ", "), strings.Join(disabledByDefaultControllers, ", "),
	))
	flags.StringVar(&o.BindAddress, "bind-address", defaultBindAddress,
		"The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.SecurePort, "secure-port", defaultPort,
		"The secure port on which to serve HTTPS.")
	flags.DurationVar(&o.ClusterStatusUpdateFrequency.Duration, "cluster-status-update-frequency", 10*time.Second,
		"Specifies how often karmada-controller-manager posts cluster status to karmada-apiserver.")
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", util.NamespaceKarmadaSystem, "The namespace of resource object that is used for locking during leader election.")
	flags.DurationVar(&o.ClusterLeaseDuration.Duration, "cluster-lease-duration", 40*time.Second,
		"Specifies the expiration period of a cluster lease.")
	flags.Float64Var(&o.ClusterLeaseRenewIntervalFraction, "cluster-lease-renew-interval-fraction", 0.25,
		"Specifies the cluster lease renew interval fraction.")
	flags.DurationVar(&o.ClusterFailureThreshold.Duration, "cluster-failure-threshold", 30*time.Second, "The duration of failure for the cluster to be considered unhealthy.")
	flags.DurationVar(&o.ClusterMonitorPeriod.Duration, "cluster-monitor-period", 5*time.Second,
		"Specifies how often karmada-controller-manager monitors cluster health status.")
	flags.DurationVar(&o.ClusterMonitorGracePeriod.Duration, "cluster-monitor-grace-period", 40*time.Second,
		"Specifies the grace period of allowing a running cluster to be unresponsive before marking it unhealthy.")
	flags.DurationVar(&o.ClusterStartupGracePeriod.Duration, "cluster-startup-grace-period", 60*time.Second,
		"Specifies the grace period of allowing a cluster to be unresponsive during startup before marking it unhealthy.")
	flags.DurationVar(&o.FailoverEvictionTimeout.Duration, "failover-eviction-timeout", 5*time.Minute,
		"Specifies the grace period for deleting scheduling result on failed clusters.")
	flags.StringVar(&o.SkippedPropagatingAPIs, "skipped-propagating-apis", "", "Semicolon separated resources that should be skipped from propagating in addition to the default skip list(cluster.karmada.io;policy.karmada.io;work.karmada.io). Supported formats are:\n"+
		"<group> for skip resources with a specific API group(e.g. networking.k8s.io),\n"+
		"<group>/<version> for skip resources with a specific API version(e.g. networking.k8s.io/v1beta1),\n"+
		"<group>/<version>/<kind>,<kind> for skip one or more specific resource(e.g. networking.k8s.io/v1beta1/Ingress,IngressClass) where the kinds are case-insensitive.")
	flags.StringSliceVar(&o.SkippedPropagatingNamespaces, "skipped-propagating-namespaces", []string{},
		"Comma-separated namespaces that should be skipped from propagating in addition to the default skipped namespaces(karmada-system, karmada-cluster, namespaces prefixed by kube- and karmada-es-).")
	flags.StringVar(&o.ClusterAPIContext, "cluster-api-context", "", "Name of the cluster context in cluster-api management cluster kubeconfig file.")
	flags.StringVar(&o.ClusterAPIKubeconfig, "cluster-api-kubeconfig", "", "Path to the cluster-api management cluster kubeconfig file.")
	flags.Float32Var(&o.ClusterAPIQPS, "cluster-api-qps", 40.0, "QPS to use while talking with cluster kube-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.IntVar(&o.ClusterAPIBurst, "cluster-api-burst", 60, "Burst to use while talking with cluster kube-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	flags.DurationVar(&o.ClusterCacheSyncTimeout.Duration, "cluster-cache-sync-timeout", util.CacheSyncTimeout, "Timeout period waiting for cluster cache to sync.")
	flags.DurationVar(&o.ResyncPeriod.Duration, "resync-period", 0, "Base frequency the informers are resynced.")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8088, :8088)")
	flags.IntVar(&o.ConcurrentClusterSyncs, "concurrent-cluster-syncs", 5, "The number of Clusters that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentClusterResourceBindingSyncs, "concurrent-clusterresourcebinding-syncs", 5, "The number of ClusterResourceBindings that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentResourceBindingSyncs, "concurrent-resourcebinding-syncs", 5, "The number of ResourceBindings that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentWorkSyncs, "concurrent-work-syncs", 5, "The number of Works that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentNamespaceSyncs, "concurrent-namespace-syncs", 1, "The number of Namespaces that are allowed to sync concurrently.")
	flags.IntVar(&o.ConcurrentResourceTemplateSyncs, "concurrent-resource-template-syncs", 5, "The number of resource templates that are allowed to sync concurrently.")

	o.RateLimiterOpts.AddFlags(flags)
	features.FeatureGate.AddFlag(flags)
}
