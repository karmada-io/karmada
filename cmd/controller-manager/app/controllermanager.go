package app

import (
	"context"
	"flag"
	"net"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	"github.com/karmada-io/karmada/pkg/clusterdiscovery/clusterapi"
	"github.com/karmada-io/karmada/pkg/detector"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

var controllers = map[string]InitFunc{}

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "karmada-controller-manager",
		Long: `The karmada controller manager runs a bunch of controllers`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}

			return Run(ctx, opts)
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "karmada-controller-manager"))
	opts.AddFlags(cmd.Flags())
	return cmd
}

func AddController(controllerName string, initFunc InitFunc) {
	controllers[controllerName] = initFunc
}

// Run runs the controller-manager with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-controller-manager version: %s", version.Get())
	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst
	controllerManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:                     gclient.NewSchema(),
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           opts.LeaderElection.ResourceName,
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     net.JoinHostPort(opts.BindAddress, strconv.Itoa(opts.SecurePort)),
		LivenessEndpointName:       "/healthz",
	})
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	if err := controllerManager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("failed to add health check endpoint: %v", err)
		return err
	}

	setupControllers(controllerManager, opts, ctx.Done())

	// blocks until the context is done.
	if err := controllerManager.Start(ctx); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

// ControllerContext defines the context object for controller
type ControllerContext struct {
	Mgr                         controllerruntime.Manager
	ObjectWatcher               objectwatcher.ObjectWatcher
	Opts                        *options.Options
	StopChan                    <-chan struct{}
	DynamicClientSet            dynamic.Interface
	OverrideManager             overridemanager.OverrideManager
	ControlPlaneInformerManager informermanager.SingleClusterInformerManager
}

// InitFunc is used to launch a particular controller.
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx ControllerContext) (enabled bool, err error)

// setupControllers initialize controllers and setup one by one.
func setupControllers(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) {
	restConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	discoverClientSet := discovery.NewDiscoveryClientForConfigOrDie(restConfig)

	overrideManager := overridemanager.New(mgr.GetClient())
	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(opts.SkippedPropagatingAPIs); err != nil {
		// The program will never go here because the parameters have been checked
		return
	}

	skippedPropagatingNamespaces := map[string]struct{}{}
	for _, ns := range opts.SkippedPropagatingNamespaces {
		skippedPropagatingNamespaces[ns] = struct{}{}
	}

	controlPlaneInformerManager := informermanager.NewSingleClusterInformerManager(dynamicClientSet, 0, stopChan)

	resourceInterpreter := resourceinterpreter.NewResourceInterpreter("", controlPlaneInformerManager)
	if err := mgr.Add(resourceInterpreter); err != nil {
		klog.Fatalf("Failed to setup custom resource interpreter: %v", err)
	}

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSet, resourceInterpreter)

	resourceDetector := &detector.ResourceDetector{
		DiscoveryClientSet:           discoverClientSet,
		Client:                       mgr.GetClient(),
		InformerManager:              controlPlaneInformerManager,
		RESTMapper:                   mgr.GetRESTMapper(),
		DynamicClient:                dynamicClientSet,
		SkippedResourceConfig:        skippedResourceConfig,
		SkippedPropagatingNamespaces: skippedPropagatingNamespaces,
		ResourceInterpreter:          resourceInterpreter,
	}
	if err := mgr.Add(resourceDetector); err != nil {
		klog.Fatalf("Failed to setup resource detector: %v", err)
	}

	setupClusterAPIClusterDetector(mgr, opts, stopChan)
	controllerContext := ControllerContext{
		Mgr:                         mgr,
		ObjectWatcher:               objectWatcher,
		Opts:                        opts,
		StopChan:                    stopChan,
		DynamicClientSet:            dynamicClientSet,
		OverrideManager:             overrideManager,
		ControlPlaneInformerManager: controlPlaneInformerManager,
	}

	if err := StartControllers(controllerContext); err != nil {
		klog.Fatalf("error starting controllers: %v", err)
	}

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-stopChan
		informermanager.StopInstance()
	}()
}

// StartControllers starts a set of controllers with a specified ControllerContext
func StartControllers(ctx ControllerContext) error {
	for controllerName, initFn := range controllers {
		if !ctx.IsControllerEnabled(controllerName) {
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

// setupClusterAPIClusterDetector initialize Cluster detector with the cluster-api management cluster.
func setupClusterAPIClusterDetector(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) {
	if len(opts.ClusterAPIKubeconfig) == 0 {
		return
	}

	klog.Infof("Begin to setup cluster-api cluster detector")

	karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())
	clusterAPIRestConfig, err := karmadaConfig.GetRestConfig(opts.ClusterAPIContext, opts.ClusterAPIKubeconfig)
	if err != nil {
		klog.Fatalf("Failed to get cluster-api management cluster rest config. context: %s, kubeconfig: %s, err: %v", opts.ClusterAPIContext, opts.ClusterAPIKubeconfig, err)
	}

	clusterAPIClient, err := gclient.NewForConfig(clusterAPIRestConfig)
	if err != nil {
		klog.Fatalf("Failed to get config from clusterAPIRestConfig: %v", err)
	}

	clusterAPIClusterDetector := &clusterapi.ClusterDetector{
		KarmadaConfig:         karmadaConfig,
		ControllerPlaneConfig: mgr.GetConfig(),
		ClusterAPIConfig:      clusterAPIRestConfig,
		ClusterAPIClient:      clusterAPIClient,
		InformerManager:       informermanager.NewSingleClusterInformerManager(dynamic.NewForConfigOrDie(clusterAPIRestConfig), 0, stopChan),
	}
	if err := mgr.Add(clusterAPIClusterDetector); err != nil {
		klog.Fatalf("Failed to setup cluster-api cluster detector: %v", err)
	}

	klog.Infof("Success to setup cluster-api cluster detector")
}

// IsControllerEnabled check if a specified controller enabled or not.
func (c ControllerContext) IsControllerEnabled(name string) bool {
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
	return hasStar
}
