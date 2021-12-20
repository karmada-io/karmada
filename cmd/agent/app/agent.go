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
	controllerscontext "github.com/karmada-io/karmada/pkg/controllers/context"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
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

	opts.AddFlags(cmd.Flags(), controllers.ControllerNames())
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "karmada-agent"))
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

var controllers = make(controllerscontext.Initializers)

func init() {
	controllers["clusterStatus"] = startClusterStatusController
	controllers["execution"] = startExecutionController
	controllers["workStatus"] = startWorkStatusController
	controllers["serviceExport"] = startServiceExportController
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
	restConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	controlPlaneInformerManager := informermanager.NewSingleClusterInformerManager(dynamicClientSet, 0, stopChan)
	resourceInterpreter := resourceinterpreter.NewResourceInterpreter("", controlPlaneInformerManager)
	if err := mgr.Add(resourceInterpreter); err != nil {
		klog.Fatalf("Failed to setup custom resource interpreter: %v", err)
	}

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSetForAgent, resourceInterpreter)
	controllerContext := controllerscontext.Context{
		Mgr:           mgr,
		ObjectWatcher: objectWatcher,
		Opts: controllerscontext.Options{
			Controllers:                       opts.Controllers,
			ClusterName:                       opts.ClusterName,
			ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
			ClusterLeaseDuration:              opts.ClusterLeaseDuration,
			ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
			ClusterCacheSyncTimeout:           opts.ClusterCacheSyncTimeout,
			ClusterAPIQPS:                     opts.ClusterAPIQPS,
			ClusterAPIBurst:                   opts.ClusterAPIBurst,
		},
		StopChan: stopChan,
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

func startClusterStatusController(ctx controllerscontext.Context) (bool, error) {
	clusterStatusController := &status.ClusterStatusController{
		Client:                            ctx.Mgr.GetClient(),
		KubeClient:                        kubeclientset.NewForConfigOrDie(ctx.Mgr.GetConfig()),
		EventRecorder:                     ctx.Mgr.GetEventRecorderFor(status.ControllerName),
		PredicateFunc:                     helper.NewClusterPredicateOnAgent(ctx.Opts.ClusterName),
		InformerManager:                   informermanager.GetInstance(),
		StopChan:                          ctx.StopChan,
		ClusterClientSetFunc:              util.NewClusterClientSetForAgent,
		ClusterDynamicClientSetFunc:       util.NewClusterDynamicClientSetForAgent,
		ClusterClientOption:               &util.ClientOption{QPS: ctx.Opts.ClusterAPIQPS, Burst: ctx.Opts.ClusterAPIBurst},
		ClusterStatusUpdateFrequency:      ctx.Opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              ctx.Opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: ctx.Opts.ClusterLeaseRenewIntervalFraction,
		ClusterCacheSyncTimeout:           ctx.Opts.ClusterCacheSyncTimeout,
	}
	if err := clusterStatusController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startExecutionController(ctx controllerscontext.Context) (bool, error) {
	executionController := &execution.Controller{
		Client:               ctx.Mgr.GetClient(),
		EventRecorder:        ctx.Mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:           ctx.Mgr.GetRESTMapper(),
		ObjectWatcher:        ctx.ObjectWatcher,
		PredicateFunc:        helper.NewExecutionPredicateOnAgent(),
		InformerManager:      informermanager.GetInstance(),
		ClusterClientSetFunc: util.NewClusterDynamicClientSetForAgent,
	}
	if err := executionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startWorkStatusController(ctx controllerscontext.Context) (bool, error) {
	workStatusController := &status.WorkStatusController{
		Client:                  ctx.Mgr.GetClient(),
		EventRecorder:           ctx.Mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:              ctx.Mgr.GetRESTMapper(),
		InformerManager:         informermanager.GetInstance(),
		StopChan:                ctx.StopChan,
		WorkerNumber:            1,
		ObjectWatcher:           ctx.ObjectWatcher,
		PredicateFunc:           helper.NewExecutionPredicateOnAgent(),
		ClusterClientSetFunc:    util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout: ctx.Opts.ClusterCacheSyncTimeout,
	}
	workStatusController.RunWorkQueue()
	if err := workStatusController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startServiceExportController(ctx controllerscontext.Context) (bool, error) {
	serviceExportController := &mcs.ServiceExportController{
		Client:                      ctx.Mgr.GetClient(),
		EventRecorder:               ctx.Mgr.GetEventRecorderFor(mcs.ServiceExportControllerName),
		RESTMapper:                  ctx.Mgr.GetRESTMapper(),
		InformerManager:             informermanager.GetInstance(),
		StopChan:                    ctx.StopChan,
		WorkerNumber:                1,
		PredicateFunc:               helper.NewPredicateForServiceExportControllerOnAgent(ctx.Opts.ClusterName),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     ctx.Opts.ClusterCacheSyncTimeout,
	}
	serviceExportController.RunWorkQueue()
	if err := serviceExportController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
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
