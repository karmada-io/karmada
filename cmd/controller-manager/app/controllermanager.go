package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/membercluster"
	"github.com/karmada-io/karmada/pkg/controllers/policy"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
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
		Scheme:           gclient.NewSchema(),
		LeaderElection:   true, // TODO(RainbowMango): Add a flag '--enable-leader-election' for this option.
		LeaderElectionID: "41db11fa.karmada.io",
	})
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
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
	karmadaClient := karmadaclientset.NewForConfigOrDie(resetConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(resetConfig)

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), kubeClientSet, mgr.GetRESTMapper())

	MemberClusterController := &membercluster.Controller{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		EventRecorder: mgr.GetEventRecorderFor(membercluster.ControllerName),
	}
	if err := MemberClusterController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup membercluster controller: %v", err)
	}

	MemberClusterStatusController := &status.MemberClusterStatusController{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		EventRecorder: mgr.GetEventRecorderFor(status.ControllerName),
	}
	if err := MemberClusterStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup memberclusterstatus controller: %v", err)
	}

	policyController := &policy.PropagationPolicyController{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClientSet,
		KarmadaClient: karmadaClient,
		EventRecorder: mgr.GetEventRecorderFor(policy.ControllerName),
		RESTMapper:    mgr.GetRESTMapper(),
	}
	if err := policyController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup policy controller: %v", err)
	}

	bindingController := &binding.PropagationBindingController{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClientSet,
		KarmadaClient: karmadaClient,
		EventRecorder: mgr.GetEventRecorderFor(binding.ControllerName),
		RESTMapper:    mgr.GetRESTMapper(),
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

	workStatusController := &status.PropagationWorkStatusController{
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
}
