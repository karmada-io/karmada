package app

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/controllers/cluster"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/hpa"
	"github.com/karmada-io/karmada/pkg/controllers/namespace"
	"github.com/karmada-io/karmada/pkg/controllers/propagationpolicy"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/detector"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(stopChan <-chan struct{}) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "controller-manager",
		Long: `The controller manager runs a bunch of controllers`,
		Run: func(cmd *cobra.Command, args []string) {
			opts.Complete()
			if err := Run(opts, stopChan); err != nil {
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
func Run(opts *options.Options, stopChan <-chan struct{}) error {
	logs.InitLogs()
	defer logs.FlushLogs()

	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	controllerManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:                 gclient.NewSchema(),
		LeaderElection:         true, // TODO(RainbowMango): Add a flag '--enable-leader-election' for this option.
		LeaderElectionID:       "41db11fa.karmada.io",
		HealthProbeBindAddress: fmt.Sprintf("%s:%d", opts.BindAddress, opts.SecurePort),
		LivenessEndpointName:   "/healthz",
	})
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	if err = controllerManager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("failed to add health check endpoint: %v", err)
		return err
	}

	setupControllers(controllerManager, stopChan)

	// blocks until the stop channel is closed.
	if err := controllerManager.Start(stopChan); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

func setupControllers(mgr controllerruntime.Manager, stopChan <-chan struct{}) {
	resetConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(resetConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(resetConfig)

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), kubeClientSet, mgr.GetRESTMapper())
	overridemanager := overridemanager.New(mgr.GetClient())

	resourceDetector := &detector.ResourceDetector{
		ClientSet:       kubeClientSet,
		InformerManager: informermanager.NewSingleClusterInformerManager(dynamicClientSet, 0),
	}
	resourceDetector.EventHandler = informermanager.NewFilteringHandlerOnAllEvents(resourceDetector.EventFilter, resourceDetector.OnAdd, resourceDetector.OnUpdate, resourceDetector.OnDelete)
	resourceDetector.Processor = util.NewAsyncWorker("resource detector", time.Second, detector.ClusterWideKeyFunc, resourceDetector.Reconcile)
	if err := mgr.Add(resourceDetector); err != nil {
		klog.Fatalf("Failed to setup resource detector: %v", err)
	}

	ClusterController := &cluster.Controller{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		EventRecorder: mgr.GetEventRecorderFor(cluster.ControllerName),
	}
	if err := ClusterController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup cluster controller: %v", err)
	}

	ClusterStatusController := &status.ClusterStatusController{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		EventRecorder: mgr.GetEventRecorderFor(status.ControllerName),
	}
	if err := ClusterStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup clusterstatus controller: %v", err)
	}

	hpaController := &hpa.HorizontalPodAutoscalerController{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClientSet,
		EventRecorder: mgr.GetEventRecorderFor(hpa.ControllerName),
		RESTMapper:    mgr.GetRESTMapper(),
	}
	if err := hpaController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup hpa controller: %v", err)
	}

	policyController := &propagationpolicy.Controller{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClientSet,
		EventRecorder: mgr.GetEventRecorderFor(propagationpolicy.ControllerName),
		RESTMapper:    mgr.GetRESTMapper(),
	}
	if err := policyController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup policy controller: %v", err)
	}

	bindingController := &binding.PropagationBindingController{
		Client:          mgr.GetClient(),
		DynamicClient:   dynamicClientSet,
		EventRecorder:   mgr.GetEventRecorderFor(binding.ControllerName),
		RESTMapper:      mgr.GetRESTMapper(),
		OverrideManager: overridemanager,
	}
	if err := bindingController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup binding controller: %v", err)
	}

	executionController := &execution.Controller{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		EventRecorder: mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:    mgr.GetRESTMapper(),
		ObjectWatcher: objectWatcher,
	}
	if err := executionController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup execution controller: %v", err)
	}

	workStatusController := &status.WorkStatusController{
		Client:          mgr.GetClient(),
		DynamicClient:   dynamicClientSet,
		EventRecorder:   mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:      mgr.GetRESTMapper(),
		KubeClientSet:   kubeClientSet,
		InformerManager: informermanager.NewMultiClusterInformerManager(),
		StopChan:        stopChan,
		WorkerNumber:    1,
		ObjectWatcher:   objectWatcher,
	}
	workStatusController.RunWorkQueue()
	if err := workStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup work status controller: %v", err)
	}

	namespaceSyncController := &namespace.Controller{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(namespace.ControllerName),
	}
	if err := namespaceSyncController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup namespace sync controller: %v", err)
	}
}
