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

package context

import (
	"context"
	"regexp"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	federatehpaconfig "github.com/karmada-io/karmada/pkg/controllers/federatedhpa/config"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// Options defines all the parameters required by our controllers.
type Options struct {
	// Controllers contains all controller names.
	Controllers []string
	// ClusterMonitorPeriod represents cluster-controller monitoring period, i.e. how often does
	// cluster-controller check cluster health signal posted from cluster-status-controller.
	// This value should be lower than ClusterMonitorGracePeriod.
	ClusterMonitorPeriod metav1.Duration
	// ClusterMonitorGracePeriod represents the grace period after last cluster health probe time.
	// If it doesn't receive update for this amount of time, it will start posting
	// "ClusterReady==ConditionUnknown".
	ClusterMonitorGracePeriod metav1.Duration
	// ClusterStartupGracePeriod specifies the grace period of allowing a cluster to be unresponsive during
	// startup before marking it unhealthy.
	ClusterStartupGracePeriod metav1.Duration
	// ClusterStatusUpdateFrequency is the frequency that controller computes and report cluster status.
	// It must work with ClusterMonitorGracePeriod.
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
	// ClusterCacheSyncTimeout is the timeout period waiting for cluster cache to sync.
	ClusterCacheSyncTimeout metav1.Duration
	// SkippedPropagatingNamespaces is a list of namespace regular expressions, matching namespaces will be skipped propagating.
	SkippedPropagatingNamespaces []*regexp.Regexp
	// ClusterName is the name of cluster.
	ClusterName string
	// ConcurrentWorkSyncs is the number of Works that are allowed to sync concurrently.
	ConcurrentWorkSyncs int
	// RateLimiterOptions contains the options for rate limiter.
	RateLimiterOptions ratelimiterflag.Options
	// If set to true enables NoExecute Taints and will evict all not-tolerating
	// objects propagating on Clusters tainted with this kind of Taints.
	EnableTaintManager bool
	// GracefulEvictionTimeout is the timeout period waiting for the grace-eviction-controller performs the final
	// removal since the workload(resource) has been moved to the graceful eviction tasks.
	GracefulEvictionTimeout metav1.Duration
	// EnableClusterResourceModeling indicates if enable cluster resource modeling.
	// The resource modeling might be used by the scheduler to make scheduling decisions
	// in scenario of dynamic replica assignment based on cluster free resources.
	// Disable if it does not fit your cases for better performance.
	EnableClusterResourceModeling bool
	// CertRotationCheckingInterval defines the interval of checking if the certificate need to be rotated.
	CertRotationCheckingInterval time.Duration
	// CertRotationRemainingTimeThreshold defines the threshold of remaining time of the valid certificate.
	// If the ratio of remaining time to total time is less than or equal to this threshold, the certificate rotation starts.
	CertRotationRemainingTimeThreshold float64
	// KarmadaKubeconfigNamespace is the namespace of the secret containing karmada-agent certificate.
	KarmadaKubeconfigNamespace string
	// HPAControllerConfiguration is the config of federatedHPA-controller.
	HPAControllerConfiguration federatehpaconfig.HPAControllerConfiguration
	// FederatedResourceQuotaOptions holds configurations for FederatedResourceQuota reconciliation.
	FederatedResourceQuotaOptions options.FederatedResourceQuotaOptions
	// FailoverConfiguration is the config of failover function.
	FailoverConfiguration options.FailoverOptions
}

// Context defines the context object for controller.
type Context struct {
	Mgr                         controllerruntime.Manager
	ObjectWatcher               objectwatcher.ObjectWatcher
	Opts                        Options
	Context                     context.Context
	DynamicClientSet            dynamic.Interface
	KubeClientSet               clientset.Interface
	OverrideManager             overridemanager.OverrideManager
	ControlPlaneInformerManager genericmanager.SingleClusterInformerManager
	ResourceInterpreter         resourceinterpreter.ResourceInterpreter
	ClusterClientOption         *util.ClientOption
}

// IsControllerEnabled check if a specified controller enabled or not.
func (c Context) IsControllerEnabled(name string, disabledByDefaultControllers sets.Set[string]) bool {
	hasStar := false
	for _, ctrl := range c.Opts.Controllers {
		if ctrl == name {
			return true
		}
		if ctrl == "-"+name {
			return false
		}
		if ctrl == "*" {
			hasStar = true
		}
	}
	// if we get here, there was no explicit choice
	if !hasStar {
		// nothing on by default
		return false
	}

	return !disabledByDefaultControllers.Has(name)
}

// InitFunc is used to launch a particular controller.
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx Context) (enabled bool, err error)

// Initializers is a public map of named controller groups
type Initializers map[string]InitFunc

// ControllerNames returns all known controller names
func (i Initializers) ControllerNames() []string {
	return sets.StringKeySet(i).List()
}

// StartControllers starts a set of controllers with a specified ControllerContext
func (i Initializers) StartControllers(ctx Context, controllersDisabledByDefault sets.Set[string]) error {
	for controllerName, initFn := range i {
		if !ctx.IsControllerEnabled(controllerName, controllersDisabledByDefault) {
			klog.Warningf("%q is disabled", controllerName)
			continue
		}
		klog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(ctx)
		if err != nil {
			klog.Errorf("Error starting %q", controllerName)
			return err
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
		klog.Infof("Started %q", controllerName)
	}
	return nil
}
