package app

import (
	"context"
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	kubeclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"

	"github.com/karmada-io/karmada/cmd/agent/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	controllerscontext "github.com/karmada-io/karmada/pkg/controllers/context"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
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

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(genericFlagSet, controllers.ControllerNames())

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.AddCommand(sharedcommand.NewCmdVersion("karmada-agent"))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

var controllers = make(controllerscontext.Initializers)

var controllersDisabledByDefault = sets.NewString()

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
		return fmt.Errorf("error building kubeconfig of karmada control plane: %w", err)
	}
	controlPlaneRestConfig.QPS, controlPlaneRestConfig.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

	clusterConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return fmt.Errorf("error building kubeconfig of member cluster: %w", err)
	}
	clusterKubeClient := kubeclientset.NewForConfigOrDie(clusterConfig)
	// ensure namespace where the impersonation secret to be stored in member cluster.
	if _, err = util.EnsureNamespaceExist(clusterKubeClient, opts.ClusterNamespace, false); err != nil {
		return err
	}

	// create a ServiceAccount for impersonator in cluster.
	impersonationSA := &corev1.ServiceAccount{}
	impersonationSA.Namespace = opts.ClusterNamespace
	impersonationSA.Name = names.GenerateServiceAccountName("impersonator")
	if _, err = util.EnsureServiceAccountExist(clusterKubeClient, impersonationSA, false); err != nil {
		return err
	}

	err = registerWithControlPlaneAPIServer(controlPlaneRestConfig, clusterConfig, opts)
	if err != nil {
		return fmt.Errorf("failed to register with karmada control plane: %w", err)
	}

	executionSpace, err := names.GenerateExecutionSpaceName(opts.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to generate execution space name for member cluster %s, err is %v", opts.ClusterName, err)
	}

	controllerManager, err := controllerruntime.NewManager(controlPlaneRestConfig, controllerruntime.Options{
		Scheme:                     gclient.NewSchema(),
		SyncPeriod:                 &opts.ResyncPeriod.Duration,
		Namespace:                  executionSpace,
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           fmt.Sprintf("karmada-agent-%s", opts.ClusterName),
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		Controller: v1alpha1.ControllerConfigurationSpec{
			GroupKindConcurrency: map[string]int{
				workv1alpha1.SchemeGroupVersion.WithKind("Work").GroupKind().String():       opts.ConcurrentWorkSyncs,
				clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster").GroupKind().String(): opts.ConcurrentClusterSyncs,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %w", err)
	}

	if err = setupControllers(controllerManager, opts, ctx.Done()); err != nil {
		return err
	}

	// blocks until the context is done.
	if err := controllerManager.Start(ctx); err != nil {
		return fmt.Errorf("controller manager exits unexpectedly: %w", err)
	}

	return nil
}

func setupControllers(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) error {
	restConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	controlPlaneInformerManager := informermanager.NewSingleClusterInformerManager(dynamicClientSet, 0, stopChan)
	resourceInterpreter := resourceinterpreter.NewResourceInterpreter("", controlPlaneInformerManager)
	if err := mgr.Add(resourceInterpreter); err != nil {
		return fmt.Errorf("failed to setup custom resource interpreter: %w", err)
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
			ConcurrentWorkSyncs:               opts.ConcurrentWorkSyncs,
			RateLimiterOptions:                opts.RateLimiterOpts,
		},
		StopChan:            stopChan,
		ResourceInterpreter: resourceInterpreter,
	}

	if err := controllers.StartControllers(controllerContext, controllersDisabledByDefault); err != nil {
		return fmt.Errorf("error starting controllers: %w", err)
	}

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-stopChan
		informermanager.StopInstance()
	}()

	return nil
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
		RateLimiterOptions:                ctx.Opts.RateLimiterOptions,
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
		RatelimiterOptions:   ctx.Opts.RateLimiterOptions,
	}
	if err := executionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startWorkStatusController(ctx controllerscontext.Context) (bool, error) {
	workStatusController := &status.WorkStatusController{
		Client:                    ctx.Mgr.GetClient(),
		EventRecorder:             ctx.Mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:                ctx.Mgr.GetRESTMapper(),
		InformerManager:           informermanager.GetInstance(),
		StopChan:                  ctx.StopChan,
		ObjectWatcher:             ctx.ObjectWatcher,
		PredicateFunc:             helper.NewExecutionPredicateOnAgent(),
		ClusterClientSetFunc:      util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:   ctx.Opts.ClusterCacheSyncTimeout,
		ConcurrentWorkStatusSyncs: ctx.Opts.ConcurrentWorkSyncs,
		RateLimiterOptions:        ctx.Opts.RateLimiterOptions,
		ResourceInterpreter:       ctx.ResourceInterpreter,
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
		WorkerNumber:                3,
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

func registerWithControlPlaneAPIServer(controlPlaneRestConfig, clusterRestConfig *restclient.Config, opts *options.Options) error {
	controlPlaneKubeClient := kubeclientset.NewForConfigOrDie(controlPlaneRestConfig)

	namespaceObj := &corev1.Namespace{}
	namespaceObj.Name = util.NamespaceClusterLease
	if _, err := util.CreateNamespace(controlPlaneKubeClient, namespaceObj); err != nil {
		klog.Errorf("Failed to create namespace(%s) object, error: %v", namespaceObj.Name, err)
		return err
	}

	impersonatorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.ClusterNamespace,
			Name:      names.GenerateImpersonationSecretName(opts.ClusterName),
		},
	}

	clusterObj, err := generateClusterInControllerPlane(controlPlaneRestConfig, opts, *impersonatorSecret)
	if err != nil {
		return err
	}

	clusterImpersonatorSecret, err := obtainCredentialsFromMemberCluster(clusterRestConfig, opts)
	if err != nil {
		return err
	}

	impersonatorSecret.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(clusterObj, clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster"))}
	impersonatorSecret.Data = map[string][]byte{
		clusterv1alpha1.SecretTokenKey: clusterImpersonatorSecret.Data[clusterv1alpha1.SecretTokenKey]}
	// create secret to store impersonation info in control plane
	_, err = util.CreateSecret(controlPlaneKubeClient, impersonatorSecret)
	if err != nil {
		return fmt.Errorf("failed to create impersonator secret in control plane. error: %v", err)
	}
	return nil
}

func generateClusterInControllerPlane(controlPlaneRestConfig *restclient.Config, opts *options.Options, impersonatorSecret corev1.Secret) (*clusterv1alpha1.Cluster, error) {
	clusterObj := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: opts.ClusterName}}
	mutateFunc := func(cluster *clusterv1alpha1.Cluster) {
		cluster.Spec.SyncMode = clusterv1alpha1.Pull
		cluster.Spec.APIEndpoint = opts.ClusterAPIEndpoint
		cluster.Spec.ProxyURL = opts.ProxyServerAddress
		cluster.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
			Namespace: impersonatorSecret.Namespace,
			Name:      impersonatorSecret.Name,
		}
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)
	cluster, err := util.CreateOrUpdateClusterObject(controlPlaneKarmadaClient, clusterObj, mutateFunc)
	if err != nil {
		klog.Errorf("Failed to create cluster(%s) object, error: %v", clusterObj.Name, err)
		return nil, err
	}

	return cluster, nil
}

func obtainCredentialsFromMemberCluster(clusterRestConfig *restclient.Config, opts *options.Options) (*corev1.Secret, error) {
	clusterKubeClient := kubeclientset.NewForConfigOrDie(clusterRestConfig)

	impersonationSA := &corev1.ServiceAccount{}
	impersonationSA.Namespace = opts.ClusterNamespace
	impersonationSA.Name = names.GenerateServiceAccountName("impersonator")
	impersonatorSecret, err := util.WaitForServiceAccountSecretCreation(clusterKubeClient, impersonationSA)
	if err != nil {
		return nil, fmt.Errorf("failed to get serviceAccount secret for impersonation from cluster(%s), error: %v", opts.ClusterName, err)
	}

	return impersonatorSecret, nil
}
