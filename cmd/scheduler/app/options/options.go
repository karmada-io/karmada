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
	"github.com/karmada-io/karmada/pkg/scheduler"
	frameworkplugins "github.com/karmada-io/karmada/pkg/scheduler/framework/plugins"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
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

	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string

	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	// It can be set to "0" or "" to disable serving the health probe.
	// Defaults to ":10351".
	HealthProbeBindAddress string

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
	// SchedulerEstimatorServiceNamespace specifies the namespace to be used for discovering scheduler estimator services.
	SchedulerEstimatorServiceNamespace string
	// SchedulerEstimatorServicePrefix presents the prefix of the accurate scheduler estimator service name.
	SchedulerEstimatorServicePrefix string
	// SchedulerEstimatorPort is the port that the accurate scheduler estimator server serves at.
	SchedulerEstimatorPort int
	// InsecureSkipEstimatorVerify controls whether verifies the grpc server's certificate chain and host name.
	InsecureSkipEstimatorVerify bool
	// SchedulerEstimatorCertFile SSL certification file used to secure scheduler estimator communication.
	SchedulerEstimatorCertFile string
	// SchedulerEstimatorKeyFile SSL key file used to secure scheduler estimator communication.
	SchedulerEstimatorKeyFile string
	// SchedulerEstimatorCaFile SSL Certificate Authority file used to secure scheduler estimator communication.
	SchedulerEstimatorCaFile string

	// EnableEmptyWorkloadPropagation represents whether workload with 0 replicas could be propagated to member clusters.
	EnableEmptyWorkloadPropagation bool
	ProfileOpts                    profileflag.Options

	// Plugins is the list of plugins to enable or disable
	// '*' means "all enabled by default plugins"
	// 'foo' means "enable 'foo'"
	// '*,-foo' means "disable 'foo'"
	Plugins []string

	// SchedulerName represents the name of the scheduler.
	// default is "default-scheduler".
	SchedulerName string

	// RateLimiterOpts contains the options for rate limiter.
	RateLimiterOpts ratelimiterflag.Options
}

// NewOptions builds an default scheduler options.
func NewOptions() *Options {
	return &Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: names.NamespaceKarmadaSystem,
			ResourceName:      names.KarmadaSchedulerComponentName,
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
	fs.StringVar(&o.LeaderElection.ResourceName, "leader-elect-resource-name", names.KarmadaSchedulerComponentName, "The name of resource object that is used for locking during leader election.")
	fs.StringVar(&o.LeaderElection.ResourceNamespace, "leader-elect-resource-namespace", names.NamespaceKarmadaSystem, "The namespace of resource object that is used for locking during leader election.")
	fs.DurationVar(&o.LeaderElection.LeaseDuration.Duration, "leader-elect-lease-duration", defaultElectionLeaseDuration.Duration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	fs.DurationVar(&o.LeaderElection.RenewDeadline.Duration, "leader-elect-renew-deadline", defaultElectionRenewDeadline.Duration, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	fs.DurationVar(&o.LeaderElection.RetryPeriod.Duration, "leader-elect-retry-period", defaultElectionRetryPeriod.Duration, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to karmada control plane kubeconfig file.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server. Overrides any value in KubeConfig. Only required if out-of-cluster.")
	fs.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the server should bind to for serving prometheus metrics(e.g. 127.0.0.1:8080, :8080). It can be set to \"0\" to disable the metrics serving. Defaults to 0.0.0.0:8080.")
	fs.StringVar(&o.HealthProbeBindAddress, "health-probe-bind-address", ":10351", "The TCP address that the server should bind to for serving health probes(e.g. 127.0.0.1:10351, :10351). It can be set to \"0\" to disable serving the health probe. Defaults to 0.0.0.0:10351.")
	fs.Float32Var(&o.KubeAPIQPS, "kube-api-qps", 40.0, "QPS to use while talking with karmada-apiserver.")
	fs.IntVar(&o.KubeAPIBurst, "kube-api-burst", 60, "Burst to use while talking with karmada-apiserver.")
	fs.BoolVar(&o.EnableSchedulerEstimator, "enable-scheduler-estimator", false, "Enable calling cluster scheduler estimator for adjusting replicas.")
	fs.BoolVar(&o.DisableSchedulerEstimatorInPullMode, "disable-scheduler-estimator-in-pull-mode", false, "Disable the scheduler estimator for clusters in pull mode, which takes effect only when enable-scheduler-estimator is true.")
	fs.DurationVar(&o.SchedulerEstimatorTimeout.Duration, "scheduler-estimator-timeout", 3*time.Second, "Specifies the timeout period of calling the scheduler estimator service.")
	fs.StringVar(&o.SchedulerEstimatorServiceNamespace, "scheduler-estimator-service-namespace", names.NamespaceKarmadaSystem, "The namespace to be used for discovering scheduler estimator services.")
	fs.StringVar(&o.SchedulerEstimatorServicePrefix, "scheduler-estimator-service-prefix", names.KarmadaSchedulerEstimatorComponentName, "The prefix of scheduler estimator service name")
	fs.IntVar(&o.SchedulerEstimatorPort, "scheduler-estimator-port", defaultEstimatorPort, "The secure port on which to connect the accurate scheduler estimator.")
	fs.StringVar(&o.SchedulerEstimatorCertFile, "scheduler-estimator-cert-file", "", "SSL certification file used to secure scheduler estimator communication.")
	fs.StringVar(&o.SchedulerEstimatorKeyFile, "scheduler-estimator-key-file", "", "SSL key file used to secure scheduler estimator communication.")
	fs.StringVar(&o.SchedulerEstimatorCaFile, "scheduler-estimator-ca-file", "", "SSL Certificate Authority file used to secure scheduler estimator communication.")
	fs.BoolVar(&o.InsecureSkipEstimatorVerify, "insecure-skip-estimator-verify", false, "Controls whether verifies the scheduler estimator's certificate chain and host name.")
	fs.BoolVar(&o.EnableEmptyWorkloadPropagation, "enable-empty-workload-propagation", false, "Enable workload with replicas 0 to be propagated to member clusters.")
	fs.StringSliceVar(&o.Plugins, "plugins", []string{"*"},
		fmt.Sprintf("A list of plugins to enable. '*' enables all build-in and customized plugins, 'foo' enables the plugin named 'foo', '*,-foo' disables the plugin named 'foo'.\nAll build-in plugins: %s.", strings.Join(frameworkplugins.NewInTreeRegistry().FactoryNames(), ",")))
	fs.StringVar(&o.SchedulerName, "scheduler-name", scheduler.DefaultScheduler, "SchedulerName represents the name of the scheduler. default is 'default-scheduler'.")
	features.FeatureGate.AddFlag(fs)
	o.ProfileOpts.AddFlags(fs)
	o.RateLimiterOpts.AddFlags(fs)
}
