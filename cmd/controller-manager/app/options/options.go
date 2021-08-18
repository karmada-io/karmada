package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

const (
	defaultMetricsBindAddress = ":8080"
	defaultBindAddress        = "0.0.0.0"
	defaultPort               = 10357
)

// Options contains everything necessary to create and run controller-manager.
type Options struct {
	HostNamespace  string
	LeaderElection componentbaseconfig.LeaderElectionConfiguration

	ManagedGroups []string
	// MetricsBindAddress is the TCP address that the controller should bind to for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	MetricsBindAddress string
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the the server serves at.
	// Note: We hope support https in the future once controller-runtime provides the functionality.
	SecurePort int
	// ClusterStatusUpdateFrequency is the frequency that controller computes and report cluster status.
	// It must work with ClusterMonitorGracePeriod(--cluster-monitor-grace-period) in karmada-controller-manager.
	ClusterStatusUpdateFrequency metav1.Duration
	// ClusterLeaseDuration is a duration that candidates for a lease need to wait to force acquire it.
	// This is measure against time of last observed lease RenewTime.
	ClusterLeaseDuration metav1.Duration
	// ClusterLeaseRenewIntervalFraction is a fraction coordinated with ClusterLeaseDuration that
	// how long the current holder of a lease has last updated the lease.
	ClusterLeaseRenewIntervalFraction float64
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
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{}
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (o *Options) Complete() {
	if len(o.HostNamespace) == 0 {
		o.HostNamespace = "default"
		klog.Infof("Set default value: Options.HostNamespace = %s", "default")
	}

	if len(o.LeaderElection.ResourceLock) == 0 {
		o.LeaderElection.ResourceLock = resourcelock.EndpointsLeasesResourceLock
		klog.Infof("Set default value: Options.LeaderElection.ResourceLock = %s", resourcelock.EndpointsLeasesResourceLock)
	}

	if o.LeaderElection.LeaseDuration.Duration.Seconds() == 0 {
		o.LeaderElection.LeaseDuration = defaultElectionLeaseDuration
		klog.Infof("Set default value: Options.LeaderElection.LeaseDuration = %s", defaultElectionLeaseDuration.Duration.String())
	}

	if o.LeaderElection.RenewDeadline.Duration.Seconds() == 0 {
		o.LeaderElection.RenewDeadline = defaultElectionRenewDeadline
		klog.Infof("Set default value: Options.LeaderElection.RenewDeadline = %s", defaultElectionRenewDeadline.Duration.String())
	}

	if o.LeaderElection.RetryPeriod.Duration.Seconds() == 0 {
		o.LeaderElection.RetryPeriod = defaultElectionRetryPeriod
		klog.Infof("Set default value: Options.LeaderElection.RetryPeriod = %s", defaultElectionRetryPeriod.Duration.String())
	}
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringSliceVar(&o.ManagedGroups, "managed-groups", []string{"autoscaling.karrier.io"}, "Groups managed by federation.")
	flags.StringVar(&o.MetricsBindAddress, "metrics-bind-address", defaultMetricsBindAddress, "The TCP address for serving prometheus metrics.")
	flags.StringVar(&o.BindAddress, "bind-address", defaultBindAddress,
		"The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.SecurePort, "secure-port", defaultPort,
		"The secure port on which to serve HTTPS.")
	flags.DurationVar(&o.ClusterStatusUpdateFrequency.Duration, "cluster-status-update-frequency", 10*time.Second,
		"Specifies how often karmada-controller-manager posts cluster status to karmada-apiserver.")
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.DurationVar(&o.ClusterLeaseDuration.Duration, "cluster-lease-duration", 40*time.Second,
		"Specifies the expiration period of a cluster lease.")
	flags.Float64Var(&o.ClusterLeaseRenewIntervalFraction, "cluster-lease-renew-interval-fraction", 0.25,
		"Specifies the cluster lease renew interval fraction.")
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
	flags.StringSliceVar(&o.SkippedPropagatingNamespaces, "skipped-propagating-namespaces", []string{},
		"Comma-separated namespaces that should be skipped from propagating in addition to the default skipped namespaces(namespaces prefixed by kube- and karmada-).")
}
