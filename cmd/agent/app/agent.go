package app

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	kubeclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/karmada-io/karmada/cmd/agent/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
)

// NewAgentCommand creates a *cobra.Command object with default parameters
func NewAgentCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()
	karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())

	cmd := &cobra.Command{
		Use:  "karmada-agent",
		Long: `The karmada agent runs the cluster registration agent`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(ctx, karmadaConfig, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	opts.AddFlags(cmd.Flags())
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

func run(ctx context.Context, karmadaConfig karmadactl.KarmadaConfig, opts *options.Options) error {
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KarmadaKubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig of karmada control plane: %s", err.Error())
	}

	err = registerWithControlPlaneAPIServer(controlPlaneRestConfig, opts.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to register with karmada control plane: %s", err.Error())
	}

	executionSpace, err := names.GenerateExecutionSpaceName(opts.ClusterName)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", opts.ClusterName, err)
		return err
	}

	controllerManager, err := controllerruntime.NewManager(controlPlaneRestConfig, controllerruntime.Options{
		Scheme:           gclient.NewSchema(),
		Namespace:        executionSpace,
		LeaderElection:   false,
		LeaderElectionID: "agent.karmada.io",
	})
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	setupControllers(controllerManager, opts, ctx.Done())

	// blocks until the context is done.
	if err := controllerManager.Start(ctx); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}

	return nil
}

func setupControllers(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) {
	clusterStatusController := &status.ClusterStatusController{
		Client:                            mgr.GetClient(),
		KubeClient:                        kubeclientset.NewForConfigOrDie(mgr.GetConfig()),
		EventRecorder:                     mgr.GetEventRecorderFor(status.ControllerName),
		PredicateFunc:                     helper.NewClusterPredicateOnAgent(opts.ClusterName),
		InformerManager:                   informermanager.GetInstance(),
		StopChan:                          stopChan,
		ClusterClientSetFunc:              util.NewClusterClientSetForAgent,
		ClusterDynamicClientSetFunc:       util.NewClusterDynamicClientSetForAgent,
		ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
	}
	if err := clusterStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup cluster status controller: %v", err)
	}

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSetForAgent)
	executionController := &execution.Controller{
		Client:               mgr.GetClient(),
		EventRecorder:        mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:           mgr.GetRESTMapper(),
		ObjectWatcher:        objectWatcher,
		PredicateFunc:        helper.NewExecutionPredicateOnAgent(),
		ClusterClientSetFunc: util.NewClusterDynamicClientSetForAgent,
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
		PredicateFunc:        helper.NewExecutionPredicateOnAgent(),
		ClusterClientSetFunc: util.NewClusterDynamicClientSetForAgent,
	}
	workStatusController.RunWorkQueue()
	if err := workStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup work status controller: %v", err)
	}

	serviceExportController := &mcs.ServiceExportController{
		Client:                      mgr.GetClient(),
		EventRecorder:               mgr.GetEventRecorderFor(mcs.ServiceExportControllerName),
		RESTMapper:                  mgr.GetRESTMapper(),
		InformerManager:             informermanager.GetInstance(),
		StopChan:                    stopChan,
		WorkerNumber:                1,
		PredicateFunc:               helper.NewPredicateForServiceExportControllerOnAgent(opts.ClusterName),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
	}
	serviceExportController.RunWorkQueue()
	if err := serviceExportController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup ServiceExport controller: %v", err)
	}
}

func registerWithControlPlaneAPIServer(controlPlaneRestConfig *restclient.Config, memberClusterName string) error {
	karmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	clusterObj := &clusterv1alpha1.Cluster{}
	clusterObj.Name = memberClusterName
	clusterObj.Spec.SyncMode = clusterv1alpha1.Pull

	_, err := karmadactl.CreateClusterObject(karmadaClient, clusterObj, false)
	if err != nil {
		klog.Errorf("failed to create cluster object. cluster name: %s, error: %v", memberClusterName, err)
		return err
	}

	return nil
}
