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
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/clusterdiscovery/clusterapi"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/controllers/cluster"
	controllerscontext "github.com/karmada-io/karmada/pkg/controllers/context"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/hpa"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	"github.com/karmada-io/karmada/pkg/controllers/namespace"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	"github.com/karmada-io/karmada/pkg/controllers/unifiedauth"
	"github.com/karmada-io/karmada/pkg/detector"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

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
	opts.AddFlags(cmd.Flags(), controllers.ControllerNames())
	return cmd
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

var controllers = make(controllerscontext.Initializers)

func init() {
	controllers["cluster"] = startClusterController
	controllers["clusterStatus"] = startClusterStatusController
	controllers["hpa"] = startHpaController
	controllers["binding"] = startBindingController
	controllers["execution"] = startExecutionController
	controllers["workStatus"] = startWorkStatusController
	controllers["namespace"] = startNamespaceController
	controllers["serviceExport"] = startServiceExportController
	controllers["endpointSlice"] = startEndpointSliceController
	controllers["serviceImport"] = startServiceImportController
	controllers["unifiedAuth"] = startUnifiedAuthController
}

func startClusterController(ctx controllerscontext.Context) (enabled bool, err error) {
	mgr := ctx.Mgr
	opts := ctx.Opts
	clusterController := &cluster.Controller{
		Client:                    mgr.GetClient(),
		EventRecorder:             mgr.GetEventRecorderFor(cluster.ControllerName),
		ClusterMonitorPeriod:      opts.ClusterMonitorPeriod.Duration,
		ClusterMonitorGracePeriod: opts.ClusterMonitorGracePeriod.Duration,
		ClusterStartupGracePeriod: opts.ClusterStartupGracePeriod.Duration,
	}
	if err := clusterController.SetupWithManager(mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startClusterStatusController(ctx controllerscontext.Context) (enabled bool, err error) {
	mgr := ctx.Mgr
	opts := ctx.Opts
	stopChan := ctx.StopChan
	clusterPredicateFunc := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			obj := createEvent.Object.(*clusterv1alpha1.Cluster)
			return obj.Spec.SyncMode == clusterv1alpha1.Push
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			obj := updateEvent.ObjectNew.(*clusterv1alpha1.Cluster)
			return obj.Spec.SyncMode == clusterv1alpha1.Push
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*clusterv1alpha1.Cluster)
			return obj.Spec.SyncMode == clusterv1alpha1.Push
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
	clusterStatusController := &status.ClusterStatusController{
		Client:                            mgr.GetClient(),
		KubeClient:                        kubeclientset.NewForConfigOrDie(mgr.GetConfig()),
		EventRecorder:                     mgr.GetEventRecorderFor(status.ControllerName),
		PredicateFunc:                     clusterPredicateFunc,
		InformerManager:                   informermanager.GetInstance(),
		StopChan:                          stopChan,
		ClusterClientSetFunc:              util.NewClusterClientSet,
		ClusterDynamicClientSetFunc:       util.NewClusterDynamicClientSet,
		ClusterClientOption:               &util.ClientOption{QPS: opts.ClusterAPIQPS, Burst: opts.ClusterAPIBurst},
		ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
		ClusterCacheSyncTimeout:           opts.ClusterCacheSyncTimeout,
	}
	if err := clusterStatusController.SetupWithManager(mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startHpaController(ctx controllerscontext.Context) (enabled bool, err error) {
	hpaController := &hpa.HorizontalPodAutoscalerController{
		Client:          ctx.Mgr.GetClient(),
		DynamicClient:   ctx.DynamicClientSet,
		EventRecorder:   ctx.Mgr.GetEventRecorderFor(hpa.ControllerName),
		RESTMapper:      ctx.Mgr.GetRESTMapper(),
		InformerManager: ctx.ControlPlaneInformerManager,
	}
	if err := hpaController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startBindingController(ctx controllerscontext.Context) (enabled bool, err error) {
	bindingController := &binding.ResourceBindingController{
		Client:              ctx.Mgr.GetClient(),
		DynamicClient:       ctx.DynamicClientSet,
		EventRecorder:       ctx.Mgr.GetEventRecorderFor(binding.ControllerName),
		RESTMapper:          ctx.Mgr.GetRESTMapper(),
		OverrideManager:     ctx.OverrideManager,
		InformerManager:     ctx.ControlPlaneInformerManager,
		ResourceInterpreter: ctx.ResourceInterpreter,
	}
	if err := bindingController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}

	clusterResourceBindingController := &binding.ClusterResourceBindingController{
		Client:              ctx.Mgr.GetClient(),
		DynamicClient:       ctx.DynamicClientSet,
		EventRecorder:       ctx.Mgr.GetEventRecorderFor(binding.ClusterResourceBindingControllerName),
		RESTMapper:          ctx.Mgr.GetRESTMapper(),
		OverrideManager:     ctx.OverrideManager,
		InformerManager:     ctx.ControlPlaneInformerManager,
		ResourceInterpreter: ctx.ResourceInterpreter,
	}
	if err := clusterResourceBindingController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startExecutionController(ctx controllerscontext.Context) (enabled bool, err error) {
	executionController := &execution.Controller{
		Client:               ctx.Mgr.GetClient(),
		EventRecorder:        ctx.Mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:           ctx.Mgr.GetRESTMapper(),
		ObjectWatcher:        ctx.ObjectWatcher,
		PredicateFunc:        helper.NewExecutionPredicate(ctx.Mgr),
		InformerManager:      informermanager.GetInstance(),
		ClusterClientSetFunc: util.NewClusterDynamicClientSet,
	}
	if err := executionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startWorkStatusController(ctx controllerscontext.Context) (enabled bool, err error) {
	opts := ctx.Opts
	workStatusController := &status.WorkStatusController{
		Client:                  ctx.Mgr.GetClient(),
		EventRecorder:           ctx.Mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:              ctx.Mgr.GetRESTMapper(),
		InformerManager:         informermanager.GetInstance(),
		StopChan:                ctx.StopChan,
		WorkerNumber:            1,
		ObjectWatcher:           ctx.ObjectWatcher,
		PredicateFunc:           helper.NewExecutionPredicate(ctx.Mgr),
		ClusterClientSetFunc:    util.NewClusterDynamicClientSet,
		ClusterCacheSyncTimeout: opts.ClusterCacheSyncTimeout,
	}
	workStatusController.RunWorkQueue()
	if err := workStatusController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startNamespaceController(ctx controllerscontext.Context) (enabled bool, err error) {
	skippedPropagatingNamespaces := map[string]struct{}{}
	for _, ns := range ctx.Opts.SkippedPropagatingNamespaces {
		skippedPropagatingNamespaces[ns] = struct{}{}
	}
	namespaceSyncController := &namespace.Controller{
		Client:                       ctx.Mgr.GetClient(),
		EventRecorder:                ctx.Mgr.GetEventRecorderFor(namespace.ControllerName),
		SkippedPropagatingNamespaces: skippedPropagatingNamespaces,
	}
	if err := namespaceSyncController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startServiceExportController(ctx controllerscontext.Context) (enabled bool, err error) {
	opts := ctx.Opts
	serviceExportController := &mcs.ServiceExportController{
		Client:                      ctx.Mgr.GetClient(),
		EventRecorder:               ctx.Mgr.GetEventRecorderFor(mcs.ServiceExportControllerName),
		RESTMapper:                  ctx.Mgr.GetRESTMapper(),
		InformerManager:             informermanager.GetInstance(),
		StopChan:                    ctx.StopChan,
		WorkerNumber:                1,
		PredicateFunc:               helper.NewPredicateForServiceExportController(ctx.Mgr),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSet,
		ClusterCacheSyncTimeout:     opts.ClusterCacheSyncTimeout,
	}
	serviceExportController.RunWorkQueue()
	if err := serviceExportController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startEndpointSliceController(ctx controllerscontext.Context) (enabled bool, err error) {
	endpointSliceController := &mcs.EndpointSliceController{
		Client:        ctx.Mgr.GetClient(),
		EventRecorder: ctx.Mgr.GetEventRecorderFor(mcs.EndpointSliceControllerName),
	}
	if err := endpointSliceController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startServiceImportController(ctx controllerscontext.Context) (enabled bool, err error) {
	serviceImportController := &mcs.ServiceImportController{
		Client:        ctx.Mgr.GetClient(),
		EventRecorder: ctx.Mgr.GetEventRecorderFor(mcs.ServiceImportControllerName),
	}
	if err := serviceImportController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startUnifiedAuthController(ctx controllerscontext.Context) (enabled bool, err error) {
	unifiedAuthController := &unifiedauth.Controller{
		Client:                ctx.Mgr.GetClient(),
		ControllerPlaneConfig: ctx.Mgr.GetConfig(),
		EventRecorder:         ctx.Mgr.GetEventRecorderFor(unifiedauth.ControllerName),
		ClusterClientSetFunc:  util.NewClusterClientSet,
	}
	if err := unifiedAuthController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

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
		EventRecorder:                mgr.GetEventRecorderFor("resource-detector"),
	}
	if err := mgr.Add(resourceDetector); err != nil {
		klog.Fatalf("Failed to setup resource detector: %v", err)
	}

	setupClusterAPIClusterDetector(mgr, opts, stopChan)
	controllerContext := controllerscontext.Context{
		Mgr:           mgr,
		ObjectWatcher: objectWatcher,
		Opts: controllerscontext.Options{
			Controllers:                       opts.Controllers,
			ClusterMonitorPeriod:              opts.ClusterMonitorPeriod,
			ClusterMonitorGracePeriod:         opts.ClusterMonitorGracePeriod,
			ClusterStartupGracePeriod:         opts.ClusterStartupGracePeriod,
			ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
			ClusterLeaseDuration:              opts.ClusterLeaseDuration,
			ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
			ClusterCacheSyncTimeout:           opts.ClusterCacheSyncTimeout,
			ClusterAPIQPS:                     opts.ClusterAPIQPS,
			ClusterAPIBurst:                   opts.ClusterAPIBurst,
			SkippedPropagatingNamespaces:      opts.SkippedPropagatingNamespaces,
		},
		StopChan:                    stopChan,
		DynamicClientSet:            dynamicClientSet,
		OverrideManager:             overrideManager,
		ControlPlaneInformerManager: controlPlaneInformerManager,
		ResourceInterpreter:         resourceInterpreter,
	}

	if err := controllers.StartControllers(controllerContext); err != nil {
		klog.Fatalf("error starting controllers: %v", err)
	}

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-stopChan
		informermanager.StopInstance()
	}()
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
