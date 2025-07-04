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

package app

import (
	"context"
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/karmada-io/karmada/cmd/agent/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/certificate"
	controllerscontext "github.com/karmada-io/karmada/pkg/controllers/context"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	"github.com/karmada-io/karmada/pkg/controllers/multiclusterservice"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	"github.com/karmada-io/karmada/pkg/features"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/typedmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewAgentCommand creates a *cobra.Command object with default parameters
func NewAgentCommand(ctx context.Context) *cobra.Command {
	logConfig := logsv1.NewLoggingConfiguration()
	fss := cliflag.NamedFlagSets{}

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	logs.AddFlags(logsFlagSet, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logConfig, logsFlagSet)
	klogflag.Add(logsFlagSet)

	genericFlagSet := fss.FlagSet("generic")
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	opts := options.NewOptions()
	opts.AddFlags(genericFlagSet, controllers.ControllerNames())

	cmd := &cobra.Command{
		Use: names.KarmadaAgentComponentName,
		Long: `The karmada-agent is the agent of member clusters. It can register a specific cluster to the Karmada control
plane and sync manifests from the Karmada control plane to the member cluster. In addition, it also syncs the status of member
cluster and manifests to the Karmada control plane.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := logsv1.ValidateAndApply(logConfig, features.FeatureGate); err != nil {
				return err
			}
			logs.InitLogs()
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(ctx, opts); err != nil {
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

	cmd.AddCommand(sharedcommand.NewCmdVersion(names.KarmadaAgentComponentName))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

var controllers = make(controllerscontext.Initializers)

var controllersDisabledByDefault = sets.New(
	"certRotation",
)

func init() {
	controllers["clusterStatus"] = startClusterStatusController
	controllers["execution"] = startExecutionController
	controllers["workStatus"] = startWorkStatusController
	controllers["serviceExport"] = startServiceExportController
	controllers["certRotation"] = startCertRotationController
	controllers["endpointsliceCollect"] = startEndpointSliceCollectController
}

func run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-agent version: %s", version.Get())

	profileflag.ListenAndServe(opts.ProfileOpts)

	controlPlaneRestConfig, err := apiclient.RestConfig(opts.KarmadaContext, opts.KarmadaKubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig of karmada control plane: %w", err)
	}
	controlPlaneRestConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(opts.KubeAPIQPS, opts.KubeAPIBurst)
	clusterConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return fmt.Errorf("error building kubeconfig of member cluster: %w", err)
	}
	clusterKubeClient := kubeclientset.NewForConfigOrDie(clusterConfig)
	controlPlaneKubeClient := kubeclientset.NewForConfigOrDie(controlPlaneRestConfig)
	karmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	registerOption := util.ClusterRegisterOption{
		ClusterNamespace:   opts.ClusterNamespace,
		ClusterName:        opts.ClusterName,
		ReportSecrets:      opts.ReportSecrets,
		ClusterAPIEndpoint: opts.ClusterAPIEndpoint,
		ProxyServerAddress: opts.ProxyServerAddress,
		ClusterProvider:    opts.ClusterProvider,
		ClusterRegion:      opts.ClusterRegion,
		ClusterZones:       opts.ClusterZones,
		DryRun:             false,
		ControlPlaneConfig: controlPlaneRestConfig,
		ClusterConfig:      clusterConfig,
	}

	registerOption.ClusterID, err = util.ObtainClusterID(clusterKubeClient)
	if err != nil {
		return err
	}

	if err = registerOption.Validate(karmadaClient, true); err != nil {
		return err
	}

	clusterSecret, impersonatorSecret, err := util.ObtainCredentialsFromMemberCluster(clusterKubeClient, registerOption)
	if err != nil {
		return err
	}
	if clusterSecret != nil {
		registerOption.Secret = *clusterSecret
	}
	if impersonatorSecret != nil {
		registerOption.ImpersonatorSecret = *impersonatorSecret
	}
	err = util.RegisterClusterInControllerPlane(registerOption, controlPlaneKubeClient, generateClusterInControllerPlane)
	if err != nil {
		return fmt.Errorf("failed to register with karmada control plane: %w", err)
	}

	executionSpace := names.GenerateExecutionSpaceName(opts.ClusterName)

	controllerManager, err := controllerruntime.NewManager(controlPlaneRestConfig, controllerruntime.Options{
		Scheme:                     gclient.NewSchema(),
		Cache:                      cache.Options{SyncPeriod: &opts.ResyncPeriod.Duration, DefaultNamespaces: map[string]cache.Config{executionSpace: {}}},
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           fmt.Sprintf("karmada-agent-%s", opts.ClusterName),
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		LeaseDuration:              &opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &opts.LeaderElection.RetryPeriod.Duration,
		HealthProbeBindAddress:     opts.HealthProbeBindAddress,
		LivenessEndpointName:       "/healthz",
		Metrics:                    metricsserver.Options{BindAddress: opts.MetricsBindAddress},
		MapperProvider:             restmapper.MapperProvider,
		BaseContext: func() context.Context {
			return ctx
		},
		Controller: config.Controller{
			GroupKindConcurrency: map[string]int{
				workv1alpha1.SchemeGroupVersion.WithKind("Work").GroupKind().String():       opts.ConcurrentWorkSyncs,
				clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster").GroupKind().String(): opts.ConcurrentClusterSyncs,
			},
			CacheSyncTimeout: opts.ClusterCacheSyncTimeout.Duration,
		},
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.DefaultTransform = fedinformer.StripUnusedFields
			return cache.New(config, opts)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build controller manager: %w", err)
	}

	if err := controllerManager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("Failed to add health check endpoint: %v", err)
		return err
	}

	ctrlmetrics.Registry.MustRegister(metrics.ClusterCollectors()...)
	ctrlmetrics.Registry.MustRegister(metrics.ResourceCollectorsForAgent()...)
	ctrlmetrics.Registry.MustRegister(metrics.PoolCollectors()...)
	ctrlmetrics.Registry.MustRegister(metrics.NewBuildInfoCollector())

	if err = setupControllers(ctx, controllerManager, opts); err != nil {
		return err
	}

	// blocks until the context is done.
	if err := controllerManager.Start(ctx); err != nil {
		return fmt.Errorf("controller manager exits unexpectedly: %w", err)
	}

	return nil
}

func setupControllers(ctx context.Context, mgr controllerruntime.Manager, opts *options.Options) error {
	restConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	controlPlaneInformerManager := genericmanager.NewSingleClusterInformerManager(ctx, dynamicClientSet, 0)
	controlPlaneKubeClientSet := kubeclientset.NewForConfigOrDie(restConfig)

	// We need a service lister to build a resource interpreter with `ClusterIPServiceResolver`
	// witch allows connection to the customized interpreter webhook without a cluster DNS service.
	sharedFactory := informers.NewSharedInformerFactory(controlPlaneKubeClientSet, 0)
	serviceLister := sharedFactory.Core().V1().Services().Lister()
	sharedFactory.Start(ctx.Done())
	sharedFactory.WaitForCacheSync(ctx.Done())

	resourceInterpreter := resourceinterpreter.NewResourceInterpreter(controlPlaneInformerManager, serviceLister)
	if err := mgr.Add(resourceInterpreter); err != nil {
		return fmt.Errorf("failed to setup custom resource interpreter: %w", err)
	}

	rateLimiterGetter := util.GetClusterRateLimiterGetter().SetDefaultLimits(opts.ClusterAPIQPS, opts.ClusterAPIBurst)
	clusterClientOption := &util.ClientOption{RateLimiterGetter: rateLimiterGetter.GetRateLimiter}

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSetForAgent, clusterClientOption, resourceInterpreter)
	controllerContext := controllerscontext.Context{
		Mgr:           mgr,
		ObjectWatcher: objectWatcher,
		Opts: controllerscontext.Options{
			Controllers:                        opts.Controllers,
			ClusterName:                        opts.ClusterName,
			ClusterStatusUpdateFrequency:       opts.ClusterStatusUpdateFrequency,
			ClusterLeaseDuration:               opts.ClusterLeaseDuration,
			ClusterLeaseRenewIntervalFraction:  opts.ClusterLeaseRenewIntervalFraction,
			ClusterSuccessThreshold:            opts.ClusterSuccessThreshold,
			ClusterFailureThreshold:            opts.ClusterFailureThreshold,
			ClusterCacheSyncTimeout:            opts.ClusterCacheSyncTimeout,
			ConcurrentWorkSyncs:                opts.ConcurrentWorkSyncs,
			RateLimiterOptions:                 opts.RateLimiterOpts,
			EnableClusterResourceModeling:      opts.EnableClusterResourceModeling,
			CertRotationCheckingInterval:       opts.CertRotationCheckingInterval,
			CertRotationRemainingTimeThreshold: opts.CertRotationRemainingTimeThreshold,
			KarmadaKubeconfigNamespace:         opts.KarmadaKubeconfigNamespace,
		},
		Context:             ctx,
		ResourceInterpreter: resourceInterpreter,
		ClusterClientOption: clusterClientOption,
	}

	if err := controllers.StartControllers(controllerContext, controllersDisabledByDefault); err != nil {
		return fmt.Errorf("error starting controllers: %w", err)
	}

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-ctx.Done()
		genericmanager.StopInstance()
	}()

	return nil
}

func startClusterStatusController(ctx controllerscontext.Context) (bool, error) {
	clusterStatusController := &status.ClusterStatusController{
		Client:                            ctx.Mgr.GetClient(),
		KubeClient:                        kubeclientset.NewForConfigOrDie(ctx.Mgr.GetConfig()),
		EventRecorder:                     ctx.Mgr.GetEventRecorderFor(status.ControllerName),
		PredicateFunc:                     helper.NewClusterPredicateOnAgent(ctx.Opts.ClusterName),
		TypedInformerManager:              typedmanager.GetInstance(),
		GenericInformerManager:            genericmanager.GetInstance(),
		ClusterClientSetFunc:              util.NewClusterClientSetForAgent,
		ClusterDynamicClientSetFunc:       util.NewClusterDynamicClientSetForAgent,
		ClusterClientOption:               ctx.ClusterClientOption,
		ClusterStatusUpdateFrequency:      ctx.Opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              ctx.Opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: ctx.Opts.ClusterLeaseRenewIntervalFraction,
		ClusterSuccessThreshold:           ctx.Opts.ClusterSuccessThreshold,
		ClusterFailureThreshold:           ctx.Opts.ClusterFailureThreshold,
		ClusterCacheSyncTimeout:           ctx.Opts.ClusterCacheSyncTimeout,
		RateLimiterOptions:                ctx.Opts.RateLimiterOptions,
		EnableClusterResourceModeling:     ctx.Opts.EnableClusterResourceModeling,
	}
	if err := clusterStatusController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startExecutionController(ctx controllerscontext.Context) (bool, error) {
	executionController := &execution.Controller{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:         ctx.Mgr.GetRESTMapper(),
		ObjectWatcher:      ctx.ObjectWatcher,
		InformerManager:    genericmanager.GetInstance(),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err := executionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startWorkStatusController(ctx controllerscontext.Context) (bool, error) {
	workStatusController := &status.WorkStatusController{
		Client:                      ctx.Mgr.GetClient(),
		EventRecorder:               ctx.Mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:                  ctx.Mgr.GetRESTMapper(),
		InformerManager:             genericmanager.GetInstance(),
		Context:                     ctx.Context,
		ObjectWatcher:               ctx.ObjectWatcher,
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     ctx.Opts.ClusterCacheSyncTimeout,
		ConcurrentWorkStatusSyncs:   ctx.Opts.ConcurrentWorkSyncs,
		RateLimiterOptions:          ctx.Opts.RateLimiterOptions,
		ResourceInterpreter:         ctx.ResourceInterpreter,
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
		InformerManager:             genericmanager.GetInstance(),
		Context:                     ctx.Context,
		WorkerNumber:                3,
		PredicateFunc:               helper.NewPredicateForServiceExportControllerOnAgent(ctx.Opts.ClusterName),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     ctx.Opts.ClusterCacheSyncTimeout,
		RateLimiterOptions:          ctx.Opts.RateLimiterOptions,
	}
	if err := indexregistry.RegisterWorkIndexByFieldSuspendDispatching(ctx.Context, ctx.Mgr); err != nil {
		return false, err
	}
	serviceExportController.RunWorkQueue()
	if err := serviceExportController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startEndpointSliceCollectController(ctx controllerscontext.Context) (enabled bool, err error) {
	if !features.FeatureGate.Enabled(features.MultiClusterService) {
		return false, nil
	}
	opts := ctx.Opts
	endpointSliceCollectController := &multiclusterservice.EndpointSliceCollectController{
		Client:                      ctx.Mgr.GetClient(),
		RESTMapper:                  ctx.Mgr.GetRESTMapper(),
		InformerManager:             genericmanager.GetInstance(),
		Context:                     ctx.Context,
		WorkerNumber:                3,
		PredicateFunc:               helper.NewPredicateForEndpointSliceCollectControllerOnAgent(opts.ClusterName),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSet,
		ClusterCacheSyncTimeout:     opts.ClusterCacheSyncTimeout,
		RateLimiterOptions:          ctx.Opts.RateLimiterOptions,
	}
	endpointSliceCollectController.RunWorkQueue()
	if err := endpointSliceCollectController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startCertRotationController(ctx controllerscontext.Context) (bool, error) {
	certRotationController := &certificate.CertRotationController{
		Client:                             ctx.Mgr.GetClient(),
		KubeClient:                         kubeclientset.NewForConfigOrDie(ctx.Mgr.GetConfig()),
		EventRecorder:                      ctx.Mgr.GetEventRecorderFor(certificate.CertRotationControllerName),
		RESTMapper:                         ctx.Mgr.GetRESTMapper(),
		ClusterClientSetFunc:               util.NewClusterClientSetForAgent,
		PredicateFunc:                      helper.NewClusterPredicateOnAgent(ctx.Opts.ClusterName),
		InformerManager:                    genericmanager.GetInstance(),
		RatelimiterOptions:                 ctx.Opts.RateLimiterOptions,
		CertRotationCheckingInterval:       ctx.Opts.CertRotationCheckingInterval,
		CertRotationRemainingTimeThreshold: ctx.Opts.CertRotationRemainingTimeThreshold,
		KarmadaKubeconfigNamespace:         ctx.Opts.KarmadaKubeconfigNamespace,
	}
	if err := certRotationController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func generateClusterInControllerPlane(opts util.ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
	clusterObj := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: opts.ClusterName}}
	mutateFunc := func(cluster *clusterv1alpha1.Cluster) {
		cluster.Spec.SyncMode = clusterv1alpha1.Pull
		cluster.Spec.APIEndpoint = opts.ClusterAPIEndpoint
		cluster.Spec.ProxyURL = opts.ProxyServerAddress
		cluster.Spec.ID = opts.ClusterID
		if opts.ClusterProvider != "" {
			cluster.Spec.Provider = opts.ClusterProvider
		}

		if len(opts.ClusterZones) > 0 {
			cluster.Spec.Zones = opts.ClusterZones
		}

		if opts.ClusterRegion != "" {
			cluster.Spec.Region = opts.ClusterRegion
		}

		cluster.Spec.InsecureSkipTLSVerification = opts.ClusterConfig.TLSClientConfig.Insecure

		if opts.ClusterConfig.Proxy != nil {
			url, err := opts.ClusterConfig.Proxy(nil)
			if err != nil {
				klog.Errorf("clusterConfig.Proxy error, %v", err)
			} else {
				cluster.Spec.ProxyURL = url.String()
			}
		}
		if opts.IsKubeCredentialsEnabled() {
			cluster.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
				Namespace: opts.Secret.Namespace,
				Name:      opts.Secret.Name,
			}
		}
		if opts.IsKubeImpersonatorEnabled() {
			cluster.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
				Namespace: opts.ImpersonatorSecret.Namespace,
				Name:      opts.ImpersonatorSecret.Name,
			}
		}
	}
	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(opts.ControlPlaneConfig)
	cluster, err := util.CreateOrUpdateClusterObject(controlPlaneKarmadaClient, clusterObj, mutateFunc)
	if err != nil {
		klog.Errorf("Failed to create cluster(%s) object, error: %v", clusterObj.Name, err)
		return nil, err
	}

	return cluster, nil
}
