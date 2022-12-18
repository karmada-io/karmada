package app

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimecfg "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/karmada-io/karmada/operator/cmd/operator/app/options"
	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	ctrlctx "github.com/karmada-io/karmada/operator/pkg/controller/context"
	"github.com/karmada-io/karmada/operator/pkg/controller/karmada"
	"github.com/karmada-io/karmada/operator/pkg/scheme"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
)

// NewOperatorCommand creates a *cobra.Command object with default parameters
func NewOperatorCommand(ctx context.Context) *cobra.Command {
	o := options.NewOptions()
	cmd := &cobra.Command{
		Use: "karmada-operator",
		PersistentPreRunE: func(*cobra.Command, []string) error {
			// silence client-go warnings.
			// karmada-operator generically watches APIs (including deprecated ones),
			// and CI ensures it works properly against matching kube-apiserver versions.
			restclient.SetDefaultWarningHandler(restclient.NoWarnings{})
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	// Add the flag(--kubeconfig) that is added by controller-runtime
	// (https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.1/pkg/client/config/config.go#L39).
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	o.AddFlags(genericFlagSet, controllers.ControllerNames(), controllersDisabledByDefault.List())

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

// Run runs the karmada-operator. This should never exit.
func Run(ctx context.Context, o *options.Options) error {
	klog.InfoS("Golang settings", "GOGC", os.Getenv("GOGC"), "GOMAXPROCS", os.Getenv("GOMAXPROCS"), "GOTRACEBACK", os.Getenv("GOTRACEBACK"))

	manager, err := createControllerManager(ctx, o)
	if err != nil {
		klog.Errorf("failed to build controller manager: %v", err)
		return err
	}

	if err := manager.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Errorf("failed to add health check endpoint: %v", err)
		return err
	}

	controllerCtx := ctrlctx.Context{
		Controllers: o.Controllers,
		Manager:     manager,
	}
	if err := controllers.StartControllers(controllerCtx, controllersDisabledByDefault); err != nil {
		klog.Errorf("failed to start controllers: %v", err)
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
var controllersDisabledByDefault = sets.NewString()

func init() {
	controllers["karmada"] = startKarmadaController
}

func startKarmadaController(ctx ctrlctx.Context) (bool, error) {
	ctrl := &karmada.Controller{
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
	config, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, err
	}

	opts := controllerruntime.Options{
		Logger: klog.Background(),
		Scheme: scheme.Scheme,
		BaseContext: func() context.Context {
			return ctx
		},
		SyncPeriod:                 &o.ResyncPeriod.Duration,
		LeaderElection:             o.LeaderElection.LeaderElect,
		LeaderElectionID:           o.LeaderElection.ResourceName,
		LeaderElectionNamespace:    o.LeaderElection.ResourceNamespace,
		LeaseDuration:              &o.LeaderElection.LeaseDuration.Duration,
		RenewDeadline:              &o.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:                &o.LeaderElection.RetryPeriod.Duration,
		LeaderElectionResourceLock: o.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     net.JoinHostPort(o.BindAddress, strconv.Itoa(o.SecurePort)),
		LivenessEndpointName:       "/healthz",
		MetricsBindAddress:         o.MetricsBindAddress,
		Controller: ctrlruntimecfg.ControllerConfigurationSpec{
			GroupKindConcurrency: map[string]int{
				operatorv1alpha1.SchemeGroupVersion.WithKind("Karmada").GroupKind().String(): o.ConcurrentKarmadaSyncs,
			},
		},
	}
	return controllerruntime.NewManager(config, opts)
}
