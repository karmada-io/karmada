package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/controllers/cluster"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	//"github.com/karmada-io/karmada/pkg/controllers/hpa"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	//"github.com/karmada-io/karmada/pkg/controllers/namespace"
	"github.com/karmada-io/karmada/pkg/controllers/propagationpolicy"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/detector"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "controller-manager",
		Long: `The controller manager runs a bunch of controllers`,
		Run: func(cmd *cobra.Command, args []string) {
			opts.Complete()
			if err := Run(ctx, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(cmd.Flags())
	return cmd
}

// Run runs the controller-manager with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	controllerManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:                 gclient.NewSchema(),
		LeaderElection:         opts.LeaderElection.LeaderElect,
		LeaderElectionID:       "karmada-controller-manager",
		HealthProbeBindAddress: fmt.Sprintf("%s:%d", opts.BindAddress, opts.SecurePort),
		LivenessEndpointName:   "/healthz",
		MetricsBindAddress:     opts.MetricsBindAddress,
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

// setupControllers initialize controllers and setup one by one.
// Note: ignore cyclomatic complexity check(by gocyclo) because it will not effect readability.
//nolint:gocyclo
func setupControllers(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) {
	resetConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(resetConfig)
	discoverClientSet := discovery.NewDiscoveryClientForConfigOrDie(resetConfig)
	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSet)
	overrideManager := overridemanager.New(mgr.GetClient())
	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(opts.SkippedPropagatingAPIs); err != nil {
		// TODO(RainbowMango): the parameter should be checked earlier.
		// Consider add validation to 'options.Options'
		panic(err)
	}
	skippedPropagatingNamespaces := map[string]struct{}{}
	for _, ns := range opts.SkippedPropagatingNamespaces {
		skippedPropagatingNamespaces[ns] = struct{}{}
	}
	resourceDetector := &detector.ResourceDetector{
		DiscoveryClientSet:           discoverClientSet,
		Client:                       mgr.GetClient(),
		InformerManager:              informermanager.NewSingleClusterInformerManager(dynamicClientSet, 0),
		RESTMapper:                   mgr.GetRESTMapper(),
		DynamicClient:                dynamicClientSet,
		SkippedResourceConfig:        skippedResourceConfig,
		SkippedPropagatingNamespaces: skippedPropagatingNamespaces,
		ManagedGroups:                opts.ManagedGroups,
	}

	resourceDetector.EventHandler = informermanager.NewFilteringHandlerOnAllEvents(resourceDetector.EventFilter, resourceDetector.OnAdd, resourceDetector.OnUpdate, resourceDetector.OnDelete)
	resourceDetector.Processor = util.NewAsyncWorker("resource detector", time.Microsecond, detector.ClusterWideKeyFunc, resourceDetector.Reconcile)
	if err := mgr.Add(resourceDetector); err != nil {
		klog.Fatalf("Failed to setup resource detector: %v", err)
	}

	clusterController := &cluster.Controller{
		Client:                    mgr.GetClient(),
		EventRecorder:             mgr.GetEventRecorderFor(cluster.ControllerName),
		ClusterMonitorPeriod:      opts.ClusterMonitorPeriod.Duration,
		ClusterMonitorGracePeriod: opts.ClusterMonitorGracePeriod.Duration,
		ClusterStartupGracePeriod: opts.ClusterStartupGracePeriod.Duration,
	}
	if err := clusterController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup cluster controller: %v", err)
	}

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
		ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
	}
	if err := clusterStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup cluster status controller: %v", err)
	}

	/*
		hpaController := &hpa.HorizontalPodAutoscalerController{
			Client:        mgr.GetClient(),
			DynamicClient: dynamicClientSet,
			EventRecorder: mgr.GetEventRecorderFor(hpa.ControllerName),
			RESTMapper:    mgr.GetRESTMapper(),
		}
		if err := hpaController.SetupWithManager(mgr); err != nil {
			klog.Fatalf("Failed to setup hpa controller: %v", err)
		}
	*/

	policyController := &propagationpolicy.Controller{
		Client: mgr.GetClient(),
	}
	if err := policyController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup policy controller: %v", err)
	}

	bindingController := &binding.ResourceBindingController{
		Client:          mgr.GetClient(),
		DynamicClient:   dynamicClientSet,
		EventRecorder:   mgr.GetEventRecorderFor(binding.ControllerName),
		RESTMapper:      mgr.GetRESTMapper(),
		OverrideManager: overrideManager,
	}
	if err := bindingController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup binding controller: %v", err)
	}

	clusterResourceBindingController := &binding.ClusterResourceBindingController{
		Client:          mgr.GetClient(),
		DynamicClient:   dynamicClientSet,
		EventRecorder:   mgr.GetEventRecorderFor(binding.ClusterResourceBindingControllerName),
		RESTMapper:      mgr.GetRESTMapper(),
		OverrideManager: overrideManager,
	}
	if err := clusterResourceBindingController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup cluster resource binding controller: %v", err)
	}

	executionController := &execution.Controller{
		Client:               mgr.GetClient(),
		EventRecorder:        mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:           mgr.GetRESTMapper(),
		ObjectWatcher:        objectWatcher,
		PredicateFunc:        helper.NewExecutionPredicate(mgr),
		ClusterClientSetFunc: util.NewClusterDynamicClientSet,
	}
	if err := executionController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup execution controller: %v", err)
	}

	workStatusController := &status.WorkStatusController{
		Client:               mgr.GetClient(),
		EventRecorder:        mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:           mgr.GetRESTMapper(),
		InformerManager:      informermanager.GetInstance(),
		StopChan:             stopChan,
		WorkerNumber:         1,
		ObjectWatcher:        objectWatcher,
		PredicateFunc:        helper.NewExecutionPredicate(mgr),
		ClusterClientSetFunc: util.NewClusterDynamicClientSet,
	}
	workStatusController.RunWorkQueue()
	if err := workStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup work status controller: %v", err)
	}

	/*
		namespaceSyncController := &namespace.Controller{
			Client:                       mgr.GetClient(),
			EventRecorder:                mgr.GetEventRecorderFor(namespace.ControllerName),
			SkippedPropagatingNamespaces: skippedPropagatingNamespaces,
		}
		if err := namespaceSyncController.SetupWithManager(mgr); err != nil {
			klog.Fatalf("Failed to setup namespace sync controller: %v", err)
		}
	*/

	serviceExportController := &mcs.ServiceExportController{
		Client:                      mgr.GetClient(),
		EventRecorder:               mgr.GetEventRecorderFor(mcs.ServiceExportControllerName),
		RESTMapper:                  mgr.GetRESTMapper(),
		InformerManager:             informermanager.GetInstance(),
		StopChan:                    stopChan,
		WorkerNumber:                1,
		PredicateFunc:               helper.NewPredicateForServiceExportController(mgr),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSet,
	}
	serviceExportController.RunWorkQueue()
	if err := serviceExportController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup ServiceExport controller: %v", err)
	}

	endpointSliceController := &mcs.EndpointSliceController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(mcs.EndpointSliceControllerName),
	}
	if err := endpointSliceController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup EndpointSlice controller: %v", err)
	}

	serviceImportController := &mcs.ServiceImportController{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(mcs.ServiceImportControllerName),
	}
	if err := serviceImportController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup ServiceImport controller: %v", err)
	}
}
