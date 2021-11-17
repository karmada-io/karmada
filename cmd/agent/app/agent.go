package app

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
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
	"github.com/karmada-io/karmada/pkg/crdexplorer"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewAgentCommand creates a *cobra.Command object with default parameters
func NewAgentCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()
	karmadaConfig := karmadactl.NewKarmadaConfig(clientcmd.NewDefaultPathOptions())

	cmd := &cobra.Command{
		Use:  "karmada-agent",
		Long: `The karmada agent runs the cluster registration agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(ctx, karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	opts.AddFlags(cmd.Flags())
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "karmada-agent"))
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

func run(ctx context.Context, karmadaConfig karmadactl.KarmadaConfig, opts *options.Options) error {
	klog.Infof("karmada-agent version: %s", version.Get())
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KarmadaKubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig of karmada control plane: %s", err.Error())
	}
	controlPlaneRestConfig.QPS, controlPlaneRestConfig.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

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
		Scheme:                     gclient.NewSchema(),
		Namespace:                  executionSpace,
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           fmt.Sprintf("karmada-agent-%s", opts.ClusterName),
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
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
		ClusterClientOption:               &util.ClientOption{QPS: opts.ClusterAPIQPS, Burst: opts.ClusterAPIBurst},
		ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
	}
	if err := clusterStatusController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup cluster status controller: %v", err)
	}

	restConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	controlPlaneInformerManager := informermanager.NewSingleClusterInformerManager(dynamicClientSet, 0, stopChan)
	crdExplorer := crdexplorer.NewCustomResourceExplorer("", controlPlaneInformerManager)
	if err := mgr.Add(crdExplorer); err != nil {
		klog.Fatalf("Failed to setup custom resource explorer: %v", err)
	}

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSetForAgent, crdExplorer)
	executionController := &execution.Controller{
		Client:          mgr.GetClient(),
		EventRecorder:   mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:      mgr.GetRESTMapper(),
		ObjectWatcher:   objectWatcher,
		PredicateFunc:   helper.NewExecutionPredicateOnAgent(),
		InformerManager: informermanager.GetInstance(),
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

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-stopChan
		informermanager.StopInstance()
	}()
}

func registerWithControlPlaneAPIServer(controlPlaneRestConfig *restclient.Config, memberClusterName string) error {
	client := gclient.NewForConfigOrDie(controlPlaneRestConfig)

	namespaceObj := &corev1.Namespace{}
	namespaceObj.Name = util.NamespaceClusterLease

	if err := util.CreateNamespaceIfNotExist(client, namespaceObj); err != nil {
		klog.Errorf("Failed to create namespace(%s) object, error: %v", namespaceObj.Name, err)
		return err
	}

	clusterObj := &clusterv1alpha1.Cluster{}
	clusterObj.Name = memberClusterName
	clusterObj.Spec.SyncMode = clusterv1alpha1.Pull

	if err := util.CreateClusterIfNotExist(client, clusterObj); err != nil {
		klog.Errorf("Failed to create cluster(%s) object, error: %v", clusterObj.Name, err)
		return err
	}

	return nil
}
