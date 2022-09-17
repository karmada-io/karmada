package options

import (
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"

	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	defaultBindAddress            = "0.0.0.0"
	defaultPort                   = 10358
	defaultEstimatorPort          = 10352
	defaultDeschedulingInterval   = 2 * time.Minute
	defaultUnschedulableThreshold = 5 * time.Minute
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

// Options contains everything necessary to create and run scheduler-estimator.
type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	KubeConfig     string
	Master         string
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the server serves at.
	SecurePort int

	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int

	// SchedulerEstimatorTimeout specifies the timeout period of calling the accurate scheduler estimator service.
	SchedulerEstimatorTimeout metav1.Duration
	// SchedulerEstimatorServicePrefix presents the prefix of the accurate scheduler estimator service name.
	SchedulerEstimatorServicePrefix string
	// SchedulerEstimatorPort is the port that the accurate scheduler estimator server serves at.
	SchedulerEstimatorPort int
	// DeschedulingInterval specifies time interval for descheduler to run.
	DeschedulingInterval metav1.Duration
	// UnschedulableThreshold specifies the period of pod unschedulable condition.
	UnschedulableThreshold metav1.Duration
	ProfileOpts            profileflag.Options
}

// NewOptions builds a default descheduler options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: util.NamespaceKarmadaSystem,
			ResourceName:      "karmada-descheduler",
			LeaseDuration:     defaultElectionLeaseDuration,
			RenewDeadline:     defaultElectionRenewDeadline,
			RetryPeriod:       defaultElectionRetryPeriod,
		},
	}
}

// AddFlags adds flags of estimator to the specified FlagSet
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Enable leader election, which must be true when running multi instances.")
	fs.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", util.NamespaceKarmadaSystem, "The namespace of resource object that is used for locking during leader election.")
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to karmada control plane kubeconfig file.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server. Overrides any value in KubeConfig. Only required if out-of-cluster.")
	fs.StringVar(&o.BindAddress, "bind-address", defaultBindAddress, "The IP address on which to listen for the --secure-port port.")
	fs.IntVar(&o.SecurePort, "secure-port", defaultPort, "The secure port on which to serve HTTPS.")
	fs.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	fs.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver. Doesn't cover events and node heartbeat apis which rate limiting is controlled by a different set of flags.")
	fs.DurationVar(&o.SchedulerEstimatorTimeout.Duration, "scheduler-estimator-timeout", 3*time.Second, "Specifies the timeout period of calling the scheduler estimator service.")
	fs.IntVar(&o.SchedulerEstimatorPort, "scheduler-estimator-port", defaultEstimatorPort, "The secure port on which to connect the accurate scheduler estimator.")
	fs.StringVar(&o.SchedulerEstimatorServicePrefix, "scheduler-estimator-service-prefix", "karmada-scheduler-estimator", "The prefix of scheduler estimator service name")
	fs.DurationVar(&o.DeschedulingInterval.Duration, "descheduling-interval", defaultDeschedulingInterval, "Time interval between two consecutive descheduler executions. Setting this value instructs the descheduler to run in a continuous loop at the interval specified.")
	fs.DurationVar(&o.UnschedulableThreshold.Duration, "unschedulable-threshold", defaultUnschedulableThreshold, "The period of pod unschedulable condition. This value is considered as a classification standard of unschedulable replicas.")
	o.ProfileOpts.AddFlags(fs)
}
