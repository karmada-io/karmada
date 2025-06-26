/*
Copyright 2022 The Karmada Authors.

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
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
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

	"github.com/karmada-io/karmada/operator/cmd/operator/app/options"
	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	ctrlctx "github.com/karmada-io/karmada/operator/pkg/controller/context"
	"github.com/karmada-io/karmada/operator/pkg/controller/karmada"
	"github.com/karmada-io/karmada/operator/pkg/scheme"
	"github.com/karmada-io/karmada/pkg/features"
	versionmetrics "github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewOperatorCommand creates a *cobra.Command object with default parameters
func NewOperatorCommand(ctx context.Context) *cobra.Command {
	o := options.NewOptions()
	logConfig := logsv1.NewLoggingConfiguration()
	fss := cliflag.NamedFlagSets{}

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	logs.AddFlags(logsFlagSet, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logConfig, logsFlagSet)
	klogflag.Add(logsFlagSet)

	genericFlagSet := fss.FlagSet("generic")
	// Add the flag(--kubeconfig) that is added by controller-runtime
	// (https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.1/pkg/client/config/config.go#L39).
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	o.AddFlags(genericFlagSet, controllers.ControllerNames(), sets.List(controllersDisabledByDefault))

	cmd := &cobra.Command{
		Use: "karmada-operator",
		PersistentPreRunE: func(*cobra.Command, []string) error {
			// silence client-go warnings.
			// karmada-operator generically watches APIs (including deprecated ones),
			// and CI ensures it works properly against matching kube-apiserver versions.
			restclient.SetDefaultWarningHandler(restclient.NoWarnings{})
			if err := logsv1.ValidateAndApply(logConfig, features.FeatureGate); err != nil {
				return err
			}
			logs.InitLogs()
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(ctx, o)
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
	cmd.AddCommand(sharedcommand.NewCmdVersion("karmada-operator"))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

// Run runs the karmada-operator. This should never exit.
func Run(ctx context.Context, o *options.Options) error {
	klog.Infof("karmada-operator version: %s", version.Get())
	klog.InfoS("Golang settings", "GOGC", os.Getenv("GOGC"), "GOMAXPROCS", os.Getenv("GOMAXPROCS"), "GOTRACEBACK", os.Getenv("GOTRACEBACK"))

	manager, err := createControllerManager(ctx, o)
	if err != nil {
		klog.Errorf("Failed to build controller manager: %v", err)
		return err
	}

	if err := manager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("Failed to add health check endpoint: %v", err)
		return err
	}

	ctrlmetrics.Registry.MustRegister(versionmetrics.NewBuildInfoCollector())

	controllerCtx := ctrlctx.Context{
		Controllers: o.Controllers,
		Manager:     manager,
	}
	if err := controllers.StartControllers(controllerCtx, controllersDisabledByDefault); err != nil {
		klog.Errorf("Failed to start controllers: %v", err)
		return err
	}

	// blocks until the context is done.
	if err := manager.Start(ctx); err != nil {
		klog.Errorf("controller manager exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

var controllers = make(ctrlctx.Initializers)

// controllersDisabledByDefault is the set of controllers which is disabled by default
var controllersDisabledByDefault = sets.New[string]()

func init() {
	controllers["karmada"] = startKarmadaController
}

func startKarmadaController(ctx ctrlctx.Context) (bool, error) {
	ctrl := &karmada.Controller{
		Config:        ctx.Manager.GetConfig(),
		Client:        ctx.Manager.GetClient(),
		EventRecorder: ctx.Manager.GetEventRecorderFor(karmada.ControllerName),
	}
	if err := ctrl.SetupWithManager(ctx.Manager); err != nil {
		klog.ErrorS(err, "unable to setup with manager", "controller", karmada.ControllerName)
		return false, err
	}
	return true, nil
}

// createControllerManager creates controllerruntime.Manager from the given configuration
func createControllerManager(ctx context.Context, o *options.Options) (controllerruntime.Manager, error) {
	restConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, err
	}

	opts := controllerruntime.Options{
		Logger: klog.Background(),
		Scheme: scheme.Scheme,
		BaseContext: func() context.Context {
			return ctx
		},
		Cache:                      cache.Options{SyncPeriod: &o.ResyncPeriod.Duration},
		LeaderElection:             o.LeaderElection.LeaderElect,
		LeaderElectionID:           o.LeaderElection.ResourceName,
		LeaderElectionNamespace:    o.LeaderElection.ResourceNamespace,
		LeaseDuration:              &o.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &o.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &o.LeaderElection.RetryPeriod.Duration,
		LeaderElectionResourceLock: o.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     o.HealthProbeBindAddress,
		LivenessEndpointName:       "/healthz",
		Metrics:                    metricsserver.Options{BindAddress: o.MetricsBindAddress},
		Controller: config.Controller{
			GroupKindConcurrency: map[string]int{
				operatorv1alpha1.SchemeGroupVersion.WithKind("Karmada").GroupKind().String(): o.ConcurrentKarmadaSyncs,
			},
		},
	}
	return controllerruntime.NewManager(restConfig, opts)
}
