package options

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
)

// Options is the main context object for the karmada-operator.
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
	// KubeAPIQPS is the QPS to use while talking with karmada-apiserver.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst to allow while talking with karmada-apiserver.
	KubeAPIBurst int32
	// ResyncPeriod is the base frequency the informers are resynced.
	// Defaults to 0, which means the created informer will never do resyncs.
	ResyncPeriod metav1.Duration
	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string
	// ConcurrentKarmadaSyncs is the number of karmada objects that are allowed to sync concurrently.
	ConcurrentKarmadaSyncs int
}

// NewOptions creates a new Options with a default config.
func NewOptions() *Options {
	o := Options{
		Controllers: []string{"*"},
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
			RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
			RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: "karmada-system",
			ResourceName:      "karmada-operator",
		},
		BindAddress:            "0.0.0.0",
		SecurePort:             8443,
		KubeAPIQPS:             50,
		KubeAPIBurst:           100,
		ConcurrentKarmadaSyncs: 5,
	}
	return &o
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet, allControllers []string, disabledByDefaultControllers []string) {
	fs.DurationVar(&o.ResyncPeriod.Duration, "resync-period", o.ResyncPeriod.Duration, "ResyncPeriod determines the minimum frequency at which watched resources are reconciled.")
	fs.Float32Var(&o.KubeAPIQPS, "kube-api-qps", o.KubeAPIQPS, "QPS to use while talking with kubernetes apiserver.")
	fs.Int32Var(&o.KubeAPIBurst, "kube-api-burst", o.KubeAPIBurst, "Burst to use while talking with kubernetes apiserver.")
	fs.StringSliceVar(&o.Controllers, "controllers", o.Controllers, fmt.Sprintf(""+
		"A list of controllers to enable. '*' enables all on-by-default controllers, 'foo' enables the controller "+
		"named 'foo', '-foo' disables the controller named 'foo'.\nAll controllers: %s .\nDisabled-by-default controllers: %s .",
		strings.Join(allControllers, ", "), strings.Join(disabledByDefaultControllers, ", ")))
	fs.IntVar(&o.ConcurrentKarmadaSyncs, "concurrent-karmada-syncs", o.ConcurrentKarmadaSyncs, "The number of karmada objects that are allowed to sync concurrently.")
	options.BindLeaderElectionFlags(&o.LeaderElection, fs)
}

// Validate is used to validate the options and config before launching the controller manager
func (o *Options) Validate() error {
	var errs []error

	// do validation logic here

	return utilerrors.NewAggregate(errs)
}
