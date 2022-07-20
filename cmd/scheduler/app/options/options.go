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
	frameworkplugins "github.com/karmada-io/karmada/pkg/scheduler/framework/plugins"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	defaultBindAddress   = "0.0.0.0"
	defaultPort          = 10351
	defaultEstimatorPort = 10352
)

var (
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

// Options contains everything necessary to create and run scheduler.
type Options struct {
	LeaderElection componentbaseconfig.LeaderElectionConfiguration
	KubeConfig     string
	Master         string
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the server serves at.
	SecurePort int

	// KubeAPIQPS is the QPS to use while talking with karmada-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int

	// EnableSchedulerEstimator represents whether the accurate scheduler estimator should be enabled.
	EnableSchedulerEstimator bool
	// DisableSchedulerEstimatorInPullMode represents whether to disable the scheduler estimator in pull mode.
	DisableSchedulerEstimatorInPullMode bool
	// SchedulerEstimatorTimeout specifies the timeout period of calling the accurate scheduler estimator service.
	SchedulerEstimatorTimeout metav1.Duration
	// SchedulerEstimatorPort is the port that the accurate scheduler estimator server serves at.
	SchedulerEstimatorPort int

	// EnableEmptyWorkloadPropagation represents whether workload with 0 replicas could be propagated to member clusters.
	EnableEmptyWorkloadPropagation bool
	ProfileOpts                    profileflag.Options

	// Plugins is the list of plugins to enable or disable
	// '*' means "all enabled by default plugins"
	// 'foo' means "enable 'foo'"
	// '*,-foo' means "disable 'foo'"
	Plugins []string
}

// NewOptions builds an default scheduler options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: util.NamespaceKarmadaSystem,
			ResourceName:      "karmada-scheduler",
			LeaseDuration:     defaultElectionLeaseDuration,
			RenewDeadline:     defaultElectionRenewDeadline,
			RetryPeriod:       defaultElectionRetryPeriod,
		},
	}
}

// AddFlags adds flags of scheduler to the specified FlagSet
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
	fs.BoolVar(&o.EnableSchedulerEstimator, "enable-scheduler-estimator", false, "Enable calling cluster scheduler estimator for adjusting replicas.")
	fs.BoolVar(&o.DisableSchedulerEstimatorInPullMode, "disable-scheduler-estimator-in-pull-mode", false, "Disable the scheduler estimator for clusters in pull mode, which takes effect only when enable-scheduler-estimator is true.")
	fs.DurationVar(&o.SchedulerEstimatorTimeout.Duration, "scheduler-estimator-timeout", 3*time.Second, "Specifies the timeout period of calling the scheduler estimator service.")
	fs.IntVar(&o.SchedulerEstimatorPort, "scheduler-estimator-port", defaultEstimatorPort, "The secure port on which to connect the accurate scheduler estimator.")
	fs.BoolVar(&o.EnableEmptyWorkloadPropagation, "enable-empty-workload-propagation", false, "Enable workload with replicas 0 to be propagated to member clusters.")
	fs.StringSliceVar(&o.Plugins, "plugins", []string{"*"},
		fmt.Sprintf("A list of plugins to enable. '*' enables all build-in and customized plugins, 'foo' enables the plugin named 'foo', '*,-foo' disables the plugin named 'foo'.\nAll build-in plugins: %s.", strings.Join(frameworkplugins.NewInTreeRegistry().FactoryNames(), ",")))
	features.FeatureGate.AddFlag(fs)
	o.ProfileOpts.AddFlags(fs)
}
