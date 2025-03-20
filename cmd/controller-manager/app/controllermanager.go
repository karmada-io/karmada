/*
Copyright 2020 The Karmada Authors.

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
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/metrics/pkg/client/custom_metrics"
	"k8s.io/metrics/pkg/client/external_metrics"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/karmada-io/karmada/cmd/controller-manager/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/clusterdiscovery/clusterapi"
	"github.com/karmada-io/karmada/pkg/controllers/applicationfailover"
	"github.com/karmada-io/karmada/pkg/controllers/binding"
	"github.com/karmada-io/karmada/pkg/controllers/certificate/approver"
	"github.com/karmada-io/karmada/pkg/controllers/cluster"
	controllerscontext "github.com/karmada-io/karmada/pkg/controllers/context"
	"github.com/karmada-io/karmada/pkg/controllers/cronfederatedhpa"
	"github.com/karmada-io/karmada/pkg/controllers/deploymentreplicassyncer"
	"github.com/karmada-io/karmada/pkg/controllers/execution"
	"github.com/karmada-io/karmada/pkg/controllers/federatedhpa"
	metricsclient "github.com/karmada-io/karmada/pkg/controllers/federatedhpa/metrics"
	"github.com/karmada-io/karmada/pkg/controllers/federatedresourcequota"
	"github.com/karmada-io/karmada/pkg/controllers/gracefuleviction"
	"github.com/karmada-io/karmada/pkg/controllers/hpascaletargetmarker"
	"github.com/karmada-io/karmada/pkg/controllers/mcs"
	"github.com/karmada-io/karmada/pkg/controllers/multiclusterservice"
	"github.com/karmada-io/karmada/pkg/controllers/namespace"
	"github.com/karmada-io/karmada/pkg/controllers/remediation"
	"github.com/karmada-io/karmada/pkg/controllers/status"
	"github.com/karmada-io/karmada/pkg/controllers/unifiedauth"
	"github.com/karmada-io/karmada/pkg/controllers/workloadrebalancer"
	"github.com/karmada-io/karmada/pkg/dependenciesdistributor"
	"github.com/karmada-io/karmada/pkg/detector"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/metrics"
	versionmetrics "github.com/karmada-io/karmada/pkg/metrics"
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
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: names.KarmadaControllerManagerComponentName,
		Long: `The karmada-controller-manager runs various controllers.
The controllers watch Karmada objects and then talk to the underlying clusters' API servers
to create regular Kubernetes resources.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}

			return Run(ctx, opts)
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	// Add the flag(--kubeconfig) that is added by controller-runtime
	// (https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.1/pkg/client/config/config.go#L39),
	// and update the flag usage.
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	genericFlagSet.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."
	opts.AddFlags(genericFlagSet, controllers.ControllerNames(), sets.List(controllersDisabledByDefault))

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.AddCommand(sharedcommand.NewCmdVersion(names.KarmadaControllerManagerComponentName))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

// Run runs the controller-manager with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-controller-manager version: %s", version.Get())

	profileflag.ListenAndServe(opts.ProfileOpts)

	controlPlaneRestConfig, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	controlPlaneRestConfig.QPS, controlPlaneRestConfig.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst
	controllerManager, err := controllerruntime.NewManager(controlPlaneRestConfig, controllerruntime.Options{
		Logger:                     klog.Background(),
		Scheme:                     gclient.NewSchema(),
		Cache:                      cache.Options{SyncPeriod: &opts.ResyncPeriod.Duration},
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           opts.LeaderElection.ResourceName,
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaseDuration:              &opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &opts.LeaderElection.RetryPeriod.Duration,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     opts.HealthProbeBindAddress,
		LivenessEndpointName:       "/healthz",
		Metrics:                    metricsserver.Options{BindAddress: opts.MetricsBindAddress},
		MapperProvider:             restmapper.MapperProvider,
		BaseContext: func() context.Context {
			return ctx
		},
		Controller: config.Controller{
			GroupKindConcurrency: map[string]int{
				workv1alpha1.SchemeGroupVersion.WithKind("Work").GroupKind().String():                     opts.ConcurrentWorkSyncs,
				workv1alpha2.SchemeGroupVersion.WithKind("ResourceBinding").GroupKind().String():          opts.ConcurrentResourceBindingSyncs,
				workv1alpha2.SchemeGroupVersion.WithKind("ClusterResourceBinding").GroupKind().String():   opts.ConcurrentClusterResourceBindingSyncs,
				clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster").GroupKind().String():               opts.ConcurrentClusterSyncs,
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}.GroupKind().String(): opts.ConcurrentNamespaceSyncs,
			},
			CacheSyncTimeout: opts.ClusterCacheSyncTimeout.Duration,
		},
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.DefaultTransform = fedinformer.StripUnusedFields
			return cache.New(config, opts)
		},
	})
	if err != nil {
		klog.Errorf("Failed to build controller manager: %v", err)
		return err
	}

	if err := controllerManager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("Failed to add health check endpoint: %v", err)
		return err
	}

	ctrlmetrics.Registry.MustRegister(metrics.ClusterCollectors()...)
	ctrlmetrics.Registry.MustRegister(metrics.ResourceCollectors()...)
	ctrlmetrics.Registry.MustRegister(metrics.PoolCollectors()...)
	ctrlmetrics.Registry.MustRegister(versionmetrics.NewBuildInfoCollector())

	if err := helper.IndexWork(ctx, controllerManager); err != nil {
		klog.Fatalf("Failed to index Work: %v", err)
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

// controllersDisabledByDefault is the set of controllers which is disabled by default
var controllersDisabledByDefault = sets.New("hpaScaleTargetMarker", "deploymentReplicasSyncer")

func init() {
	controllers["cluster"] = startClusterController
	controllers["clusterStatus"] = startClusterStatusController
	controllers["binding"] = startBindingController
	controllers["bindingStatus"] = startBindingStatusController
	controllers["execution"] = startExecutionController
	controllers["workStatus"] = startWorkStatusController
	controllers["namespace"] = startNamespaceController
	controllers["serviceExport"] = startServiceExportController
	controllers["endpointSlice"] = startEndpointSliceController
	controllers["serviceImport"] = startServiceImportController
	controllers["unifiedAuth"] = startUnifiedAuthController
	controllers["federatedResourceQuotaSync"] = startFederatedResourceQuotaSyncController
	controllers["federatedResourceQuotaStatus"] = startFederatedResourceQuotaStatusController
	controllers["gracefulEviction"] = startGracefulEvictionController
	controllers["applicationFailover"] = startApplicationFailoverController
	controllers["federatedHorizontalPodAutoscaler"] = startFederatedHorizontalPodAutoscalerController
	controllers["cronFederatedHorizontalPodAutoscaler"] = startCronFederatedHorizontalPodAutoscalerController
	controllers["hpaScaleTargetMarker"] = startHPAScaleTargetMarkerController
	controllers["deploymentReplicasSyncer"] = startDeploymentReplicasSyncerController
	controllers["multiclusterservice"] = startMCSController
	controllers["endpointsliceCollect"] = startEndpointSliceCollectController
	controllers["endpointsliceDispatch"] = startEndpointSliceDispatchController
	controllers["remedy"] = startRemedyController
	controllers["workloadRebalancer"] = startWorkloadRebalancerController
	controllers["agentcsrapproving"] = startAgentCSRApprovingController
}

func startClusterController(ctx controllerscontext.Context) (enabled bool, err error) {
	mgr := ctx.Mgr
	opts := ctx.Opts

	clusterController := &cluster.Controller{
		Client:                             mgr.GetClient(),
		EventRecorder:                      mgr.GetEventRecorderFor(cluster.ControllerName),
		ClusterMonitorPeriod:               opts.ClusterMonitorPeriod.Duration,
		ClusterMonitorGracePeriod:          opts.ClusterMonitorGracePeriod.Duration,
		ClusterStartupGracePeriod:          opts.ClusterStartupGracePeriod.Duration,
		FailoverEvictionTimeout:            opts.FailoverEvictionTimeout.Duration,
		EnableTaintManager:                 ctx.Opts.EnableTaintManager,
		ClusterTaintEvictionRetryFrequency: 10 * time.Second,
		ExecutionSpaceRetryFrequency:       10 * time.Second,
		RateLimiterOptions:                 ctx.Opts.RateLimiterOptions,
	}
	if err := clusterController.SetupWithManager(mgr); err != nil {
		return false, err
	}

	if ctx.Opts.EnableTaintManager {
		if err := cluster.IndexField(mgr); err != nil {
			return false, err
		}
		taintManager := &cluster.NoExecuteTaintManager{
			Client:                             mgr.GetClient(),
			EventRecorder:                      mgr.GetEventRecorderFor(cluster.TaintManagerName),
			ClusterTaintEvictionRetryFrequency: 10 * time.Second,
			ConcurrentReconciles:               3,
			RateLimiterOptions:                 ctx.Opts.RateLimiterOptions,
		}
		if err := taintManager.SetupWithManager(mgr); err != nil {
			return false, err
		}
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

			if obj.Spec.SecretRef == nil {
				return false
			}

			return obj.Spec.SyncMode == clusterv1alpha1.Push
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			obj := updateEvent.ObjectNew.(*clusterv1alpha1.Cluster)

			if obj.Spec.SecretRef == nil {
				return false
			}

			return obj.Spec.SyncMode == clusterv1alpha1.Push
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*clusterv1alpha1.Cluster)

			if obj.Spec.SecretRef == nil {
				return false
			}

			return obj.Spec.SyncMode == clusterv1alpha1.Push
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
	clusterStatusController := &status.ClusterStatusController{
		Client:                            mgr.GetClient(),
		KubeClient:                        kubeclientset.NewForConfigOrDie(mgr.GetConfig()),
		EventRecorder:                     mgr.GetEventRecorderFor(status.ControllerName),
		PredicateFunc:                     clusterPredicateFunc,
		TypedInformerManager:              typedmanager.GetInstance(),
		GenericInformerManager:            genericmanager.GetInstance(),
		StopChan:                          stopChan,
		ClusterClientSetFunc:              util.NewClusterClientSet,
		ClusterDynamicClientSetFunc:       util.NewClusterDynamicClientSet,
		ClusterClientOption:               &util.ClientOption{QPS: opts.ClusterAPIQPS, Burst: opts.ClusterAPIBurst},
		ClusterStatusUpdateFrequency:      opts.ClusterStatusUpdateFrequency,
		ClusterLeaseDuration:              opts.ClusterLeaseDuration,
		ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
		ClusterSuccessThreshold:           opts.ClusterSuccessThreshold,
		ClusterFailureThreshold:           opts.ClusterFailureThreshold,
		ClusterCacheSyncTimeout:           opts.ClusterCacheSyncTimeout,
		RateLimiterOptions:                ctx.Opts.RateLimiterOptions,
		EnableClusterResourceModeling:     ctx.Opts.EnableClusterResourceModeling,
	}
	if err := clusterStatusController.SetupWithManager(mgr); err != nil {
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
		RateLimiterOptions:  ctx.Opts.RateLimiterOptions,
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
		RateLimiterOptions:  ctx.Opts.RateLimiterOptions,
	}
	if err := clusterResourceBindingController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startBindingStatusController(ctx controllerscontext.Context) (enabled bool, err error) {
	rbStatusController := &status.RBStatusController{
		Client:              ctx.Mgr.GetClient(),
		DynamicClient:       ctx.DynamicClientSet,
		InformerManager:     ctx.ControlPlaneInformerManager,
		ResourceInterpreter: ctx.ResourceInterpreter,
		EventRecorder:       ctx.Mgr.GetEventRecorderFor(status.RBStatusControllerName),
		RESTMapper:          ctx.Mgr.GetRESTMapper(),
		RateLimiterOptions:  ctx.Opts.RateLimiterOptions,
	}
	if err := rbStatusController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}

	crbStatusController := &status.CRBStatusController{
		Client:              ctx.Mgr.GetClient(),
		DynamicClient:       ctx.DynamicClientSet,
		InformerManager:     ctx.ControlPlaneInformerManager,
		ResourceInterpreter: ctx.ResourceInterpreter,
		EventRecorder:       ctx.Mgr.GetEventRecorderFor(status.CRBStatusControllerName),
		RESTMapper:          ctx.Mgr.GetRESTMapper(),
		RateLimiterOptions:  ctx.Opts.RateLimiterOptions,
	}
	if err := crbStatusController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}

	return true, nil
}

func startExecutionController(ctx controllerscontext.Context) (enabled bool, err error) {
	executionController := &execution.Controller{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(execution.ControllerName),
		RESTMapper:         ctx.Mgr.GetRESTMapper(),
		ObjectWatcher:      ctx.ObjectWatcher,
		PredicateFunc:      helper.NewExecutionPredicate(ctx.Mgr),
		InformerManager:    genericmanager.GetInstance(),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err := executionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startWorkStatusController(ctx controllerscontext.Context) (enabled bool, err error) {
	opts := ctx.Opts
	workStatusController := &status.WorkStatusController{
		Client:                      ctx.Mgr.GetClient(),
		EventRecorder:               ctx.Mgr.GetEventRecorderFor(status.WorkStatusControllerName),
		RESTMapper:                  ctx.Mgr.GetRESTMapper(),
		InformerManager:             genericmanager.GetInstance(),
		StopChan:                    ctx.StopChan,
		ObjectWatcher:               ctx.ObjectWatcher,
		PredicateFunc:               helper.NewExecutionPredicate(ctx.Mgr),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSet,
		ClusterCacheSyncTimeout:     opts.ClusterCacheSyncTimeout,
		ConcurrentWorkStatusSyncs:   opts.ConcurrentWorkSyncs,
		RateLimiterOptions:          ctx.Opts.RateLimiterOptions,
		ResourceInterpreter:         ctx.ResourceInterpreter,
	}
	workStatusController.RunWorkQueue()
	if err := workStatusController.SetupWithManager(ctx.Mgr); err != nil {
		klog.Fatalf("Failed to setup work status controller: %v", err)
		return false, err
	}
	return true, nil
}

func startNamespaceController(ctx controllerscontext.Context) (enabled bool, err error) {
	namespaceSyncController := &namespace.Controller{
		Client:                       ctx.Mgr.GetClient(),
		EventRecorder:                ctx.Mgr.GetEventRecorderFor(namespace.ControllerName),
		SkippedPropagatingNamespaces: ctx.Opts.SkippedPropagatingNamespaces,
		OverrideManager:              ctx.OverrideManager,
		RateLimiterOptions:           ctx.Opts.RateLimiterOptions,
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
		InformerManager:             genericmanager.GetInstance(),
		StopChan:                    ctx.StopChan,
		WorkerNumber:                3,
		PredicateFunc:               helper.NewPredicateForServiceExportController(ctx.Mgr),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSet,
		ClusterCacheSyncTimeout:     opts.ClusterCacheSyncTimeout,
		RateLimiterOptions:          ctx.Opts.RateLimiterOptions,
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
		StopChan:                    ctx.StopChan,
		WorkerNumber:                3,
		PredicateFunc:               helper.NewPredicateForEndpointSliceCollectController(ctx.Mgr),
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

func startEndpointSliceDispatchController(ctx controllerscontext.Context) (enabled bool, err error) {
	if !features.FeatureGate.Enabled(features.MultiClusterService) {
		return false, nil
	}
	endpointSliceSyncController := &multiclusterservice.EndpointsliceDispatchController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(multiclusterservice.EndpointsliceDispatchControllerName),
		RESTMapper:         ctx.Mgr.GetRESTMapper(),
		InformerManager:    genericmanager.GetInstance(),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err := endpointSliceSyncController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startEndpointSliceController(ctx controllerscontext.Context) (enabled bool, err error) {
	endpointSliceController := &mcs.EndpointSliceController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(mcs.EndpointSliceControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err := endpointSliceController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startServiceImportController(ctx controllerscontext.Context) (enabled bool, err error) {
	serviceImportController := &mcs.ServiceImportController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(mcs.ServiceImportControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err := serviceImportController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startUnifiedAuthController(ctx controllerscontext.Context) (enabled bool, err error) {
	unifiedAuthController := &unifiedauth.Controller{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(unifiedauth.ControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err := unifiedAuthController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startFederatedResourceQuotaSyncController(ctx controllerscontext.Context) (enabled bool, err error) {
	controller := federatedresourcequota.SyncController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(federatedresourcequota.SyncControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err = controller.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startFederatedResourceQuotaStatusController(ctx controllerscontext.Context) (enabled bool, err error) {
	controller := federatedresourcequota.StatusController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(federatedresourcequota.StatusControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err = controller.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startGracefulEvictionController(ctx controllerscontext.Context) (enabled bool, err error) {
	rbGracefulEvictionController := &gracefuleviction.RBGracefulEvictionController{
		Client:                  ctx.Mgr.GetClient(),
		EventRecorder:           ctx.Mgr.GetEventRecorderFor(gracefuleviction.RBGracefulEvictionControllerName),
		RateLimiterOptions:      ctx.Opts.RateLimiterOptions,
		GracefulEvictionTimeout: ctx.Opts.GracefulEvictionTimeout.Duration,
	}
	if err := rbGracefulEvictionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}

	crbGracefulEvictionController := &gracefuleviction.CRBGracefulEvictionController{
		Client:                  ctx.Mgr.GetClient(),
		EventRecorder:           ctx.Mgr.GetEventRecorderFor(gracefuleviction.CRBGracefulEvictionControllerName),
		RateLimiterOptions:      ctx.Opts.RateLimiterOptions,
		GracefulEvictionTimeout: ctx.Opts.GracefulEvictionTimeout.Duration,
	}
	if err := crbGracefulEvictionController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}

	return true, nil
}

func startApplicationFailoverController(ctx controllerscontext.Context) (enabled bool, err error) {
	rbApplicationFailoverController := applicationfailover.RBApplicationFailoverController{
		Client:              ctx.Mgr.GetClient(),
		EventRecorder:       ctx.Mgr.GetEventRecorderFor(applicationfailover.RBApplicationFailoverControllerName),
		ResourceInterpreter: ctx.ResourceInterpreter,
		RateLimiterOptions:  ctx.Opts.RateLimiterOptions,
	}
	if err = rbApplicationFailoverController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}

	crbApplicationFailoverController := applicationfailover.CRBApplicationFailoverController{
		Client:              ctx.Mgr.GetClient(),
		EventRecorder:       ctx.Mgr.GetEventRecorderFor(applicationfailover.CRBApplicationFailoverControllerName),
		ResourceInterpreter: ctx.ResourceInterpreter,
		RateLimiterOptions:  ctx.Opts.RateLimiterOptions,
	}
	if err = crbApplicationFailoverController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startFederatedHorizontalPodAutoscalerController(ctx controllerscontext.Context) (enabled bool, err error) {
	apiVersionsGetter := custom_metrics.NewAvailableAPIsGetter(ctx.KubeClientSet.Discovery())
	go custom_metrics.PeriodicallyInvalidate(
		apiVersionsGetter,
		ctx.Opts.HPAControllerConfiguration.HorizontalPodAutoscalerSyncPeriod.Duration,
		ctx.StopChan)
	metricsClient := metricsclient.NewRESTMetricsClient(
		resourceclient.NewForConfigOrDie(ctx.Mgr.GetConfig()),
		custom_metrics.NewForConfig(ctx.Mgr.GetConfig(), ctx.Mgr.GetRESTMapper(), apiVersionsGetter),
		external_metrics.NewForConfigOrDie(ctx.Mgr.GetConfig()),
	)
	replicaCalculator := federatedhpa.NewReplicaCalculator(metricsClient,
		ctx.Opts.HPAControllerConfiguration.HorizontalPodAutoscalerTolerance,
		ctx.Opts.HPAControllerConfiguration.HorizontalPodAutoscalerCPUInitializationPeriod.Duration,
		ctx.Opts.HPAControllerConfiguration.HorizontalPodAutoscalerInitialReadinessDelay.Duration)
	federatedHPAController := federatedhpa.FHPAController{
		Client:                            ctx.Mgr.GetClient(),
		EventRecorder:                     ctx.Mgr.GetEventRecorderFor(federatedhpa.ControllerName),
		RESTMapper:                        ctx.Mgr.GetRESTMapper(),
		DownscaleStabilisationWindow:      ctx.Opts.HPAControllerConfiguration.HorizontalPodAutoscalerDownscaleStabilizationWindow.Duration,
		HorizontalPodAutoscalerSyncPeriod: ctx.Opts.HPAControllerConfiguration.HorizontalPodAutoscalerSyncPeriod.Duration,
		ReplicaCalc:                       replicaCalculator,
		ClusterScaleClientSetFunc:         util.NewClusterScaleClientSet,
		TypedInformerManager:              typedmanager.GetInstance(),
		RateLimiterOptions:                ctx.Opts.RateLimiterOptions,
		ClusterCacheSyncTimeout:           ctx.Opts.ClusterCacheSyncTimeout,
	}
	if err = federatedHPAController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startCronFederatedHorizontalPodAutoscalerController(ctx controllerscontext.Context) (enabled bool, err error) {
	cronFHPAController := cronfederatedhpa.CronFHPAController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(cronfederatedhpa.ControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err = cronFHPAController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startHPAScaleTargetMarkerController(ctx controllerscontext.Context) (enabled bool, err error) {
	hpaScaleTargetMarker := hpascaletargetmarker.HpaScaleTargetMarker{
		DynamicClient:      ctx.DynamicClientSet,
		RESTMapper:         ctx.Mgr.GetRESTMapper(),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	err = hpaScaleTargetMarker.SetupWithManager(ctx.Mgr)
	if err != nil {
		return false, err
	}

	return true, nil
}

func startDeploymentReplicasSyncerController(ctx controllerscontext.Context) (enabled bool, err error) {
	deploymentReplicasSyncer := deploymentreplicassyncer.DeploymentReplicasSyncer{
		Client:             ctx.Mgr.GetClient(),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	err = deploymentReplicasSyncer.SetupWithManager(ctx.Mgr)
	if err != nil {
		return false, err
	}

	return true, nil
}

func startMCSController(ctx controllerscontext.Context) (enabled bool, err error) {
	if !features.FeatureGate.Enabled(features.MultiClusterService) {
		return false, nil
	}
	mcsController := &multiclusterservice.MCSController{
		Client:             ctx.Mgr.GetClient(),
		EventRecorder:      ctx.Mgr.GetEventRecorderFor(multiclusterservice.ControllerName),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	if err = mcsController.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startRemedyController(ctx controllerscontext.Context) (enabled bool, err error) {
	c := &remediation.RemedyController{
		Client:           ctx.Mgr.GetClient(),
		RateLimitOptions: ctx.Opts.RateLimiterOptions,
	}
	if err = c.SetupWithManager(ctx.Mgr); err != nil {
		return false, err
	}
	return true, nil
}

func startWorkloadRebalancerController(ctx controllerscontext.Context) (enabled bool, err error) {
	workloadRebalancer := workloadrebalancer.RebalancerController{
		Client:             ctx.Mgr.GetClient(),
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	err = workloadRebalancer.SetupWithManager(ctx.Mgr)
	if err != nil {
		return false, err
	}

	return true, nil
}

func startAgentCSRApprovingController(ctx controllerscontext.Context) (enabled bool, err error) {
	agentCSRApprover := approver.AgentCSRApprovingController{
		Client:             ctx.KubeClientSet,
		RateLimiterOptions: ctx.Opts.RateLimiterOptions,
	}
	err = agentCSRApprover.SetupWithManager(ctx.Mgr)
	if err != nil {
		return false, err
	}
	return true, nil
}

// setupControllers initialize controllers and setup one by one.
func setupControllers(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) {
	restConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	discoverClientSet := discovery.NewDiscoveryClientForConfigOrDie(restConfig)
	kubeClientSet := kubeclientset.NewForConfigOrDie(restConfig)

	overrideManager := overridemanager.New(mgr.GetClient(), mgr.GetEventRecorderFor(overridemanager.OverrideManagerName))
	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(opts.SkippedPropagatingAPIs); err != nil {
		// The program will never go here because the parameters have been checked
		return
	}

	controlPlaneInformerManager := genericmanager.NewSingleClusterInformerManager(dynamicClientSet, opts.ResyncPeriod.Duration, stopChan)
	// We need a service lister to build a resource interpreter with `ClusterIPServiceResolver`
	// witch allows connection to the customized interpreter webhook without a cluster DNS service.
	sharedFactory := informers.NewSharedInformerFactory(kubeClientSet, opts.ResyncPeriod.Duration)
	serviceLister := sharedFactory.Core().V1().Services().Lister()
	sharedFactory.Start(stopChan)
	sharedFactory.WaitForCacheSync(stopChan)

	resourceInterpreter := resourceinterpreter.NewResourceInterpreter(controlPlaneInformerManager, serviceLister)
	if err := mgr.Add(resourceInterpreter); err != nil {
		klog.Fatalf("Failed to setup custom resource interpreter: %v", err)
	}

	objectWatcher := objectwatcher.NewObjectWatcher(mgr.GetClient(), mgr.GetRESTMapper(), util.NewClusterDynamicClientSet, resourceInterpreter)

	resourceDetector := &detector.ResourceDetector{
		DiscoveryClientSet:                      discoverClientSet,
		Client:                                  mgr.GetClient(),
		InformerManager:                         controlPlaneInformerManager,
		RESTMapper:                              mgr.GetRESTMapper(),
		DynamicClient:                           dynamicClientSet,
		SkippedResourceConfig:                   skippedResourceConfig,
		SkippedPropagatingNamespaces:            opts.SkippedNamespacesRegexps(),
		ResourceInterpreter:                     resourceInterpreter,
		EventRecorder:                           mgr.GetEventRecorderFor("resource-detector"),
		ConcurrentPropagationPolicySyncs:        opts.ConcurrentPropagationPolicySyncs,
		ConcurrentClusterPropagationPolicySyncs: opts.ConcurrentClusterPropagationPolicySyncs,
		ConcurrentResourceTemplateSyncs:         opts.ConcurrentResourceTemplateSyncs,
		RateLimiterOptions:                      opts.RateLimiterOpts,
	}

	if err := mgr.Add(resourceDetector); err != nil {
		klog.Fatalf("Failed to setup resource detector: %v", err)
	}
	if features.FeatureGate.Enabled(features.PropagateDeps) {
		dependenciesDistributor := &dependenciesdistributor.DependenciesDistributor{
			Client:                           mgr.GetClient(),
			DynamicClient:                    dynamicClientSet,
			InformerManager:                  controlPlaneInformerManager,
			ResourceInterpreter:              resourceInterpreter,
			RESTMapper:                       mgr.GetRESTMapper(),
			EventRecorder:                    mgr.GetEventRecorderFor("dependencies-distributor"),
			RateLimiterOptions:               opts.RateLimiterOpts,
			ConcurrentDependentResourceSyncs: opts.ConcurrentDependentResourceSyncs,
		}
		if err := dependenciesDistributor.SetupWithManager(mgr); err != nil {
			klog.Fatalf("Failed to setup dependencies distributor: %v", err)
		}
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
			FailoverEvictionTimeout:           opts.FailoverEvictionTimeout,
			ClusterLeaseDuration:              opts.ClusterLeaseDuration,
			ClusterLeaseRenewIntervalFraction: opts.ClusterLeaseRenewIntervalFraction,
			ClusterSuccessThreshold:           opts.ClusterSuccessThreshold,
			ClusterFailureThreshold:           opts.ClusterFailureThreshold,
			ClusterCacheSyncTimeout:           opts.ClusterCacheSyncTimeout,
			ClusterAPIQPS:                     opts.ClusterAPIQPS,
			ClusterAPIBurst:                   opts.ClusterAPIBurst,
			SkippedPropagatingNamespaces:      opts.SkippedNamespacesRegexps(),
			ConcurrentWorkSyncs:               opts.ConcurrentWorkSyncs,
			EnableTaintManager:                opts.EnableTaintManager,
			RateLimiterOptions:                opts.RateLimiterOpts,
			GracefulEvictionTimeout:           opts.GracefulEvictionTimeout,
			EnableClusterResourceModeling:     opts.EnableClusterResourceModeling,
			HPAControllerConfiguration:        opts.HPAControllerConfiguration,
		},
		StopChan:                    stopChan,
		DynamicClientSet:            dynamicClientSet,
		KubeClientSet:               kubeClientSet,
		OverrideManager:             overrideManager,
		ControlPlaneInformerManager: controlPlaneInformerManager,
		ResourceInterpreter:         resourceInterpreter,
	}

	if err := controllers.StartControllers(controllerContext, controllersDisabledByDefault); err != nil {
		klog.Fatalf("error starting controllers: %v", err)
	}

	// Ensure the InformerManager stops when the stop channel closes
	go func() {
		<-stopChan
		genericmanager.StopInstance()
	}()
}

// setupClusterAPIClusterDetector initialize Cluster detector with the cluster-api management cluster.
func setupClusterAPIClusterDetector(mgr controllerruntime.Manager, opts *options.Options, stopChan <-chan struct{}) {
	if len(opts.ClusterAPIKubeconfig) == 0 {
		return
	}

	klog.Infof("Begin to setup cluster-api cluster detector")

	clusterAPIRestConfig, err := apiclient.RestConfig(opts.ClusterAPIContext, opts.ClusterAPIKubeconfig)
	if err != nil {
		klog.Fatalf("Failed to get cluster-api management cluster rest config. context: %s, kubeconfig: %s, err: %v", opts.ClusterAPIContext, opts.ClusterAPIKubeconfig, err)
	}

	clusterAPIClient, err := gclient.NewForConfig(clusterAPIRestConfig)
	if err != nil {
		klog.Fatalf("Failed to get config from clusterAPIRestConfig: %v", err)
	}

	clusterAPIClusterDetector := &clusterapi.ClusterDetector{
		ControllerPlaneConfig: mgr.GetConfig(),
		ClusterAPIConfig:      clusterAPIRestConfig,
		ClusterAPIClient:      clusterAPIClient,
		InformerManager:       genericmanager.NewSingleClusterInformerManager(dynamic.NewForConfigOrDie(clusterAPIRestConfig), 0, stopChan),
		ConcurrentReconciles:  3,
	}
	if err := mgr.Add(clusterAPIClusterDetector); err != nil {
		klog.Fatalf("Failed to setup cluster-api cluster detector: %v", err)
	}

	klog.Infof("Success to setup cluster-api cluster detector")
}
