package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/karmada-io/karmada/cmd/scheduler/app/options"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/scheduler"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

const (
	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	// References:
	// - https://en.wikipedia.org/wiki/Slowloris_(computer_security)
	ReadHeaderTimeout = 32 * time.Second
	// WriteTimeout is the amount of time allowed to write the
	// request data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	WriteTimeout = 5 * time.Minute
	// ReadTimeout is the amount of time allowed to read
	// response data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	ReadTimeout = 5 * time.Minute
)

// Option configures a framework.Registry.
type Option func(runtime.Registry) error

// WithPlugin used to register a PluginFactory.
func WithPlugin(name string, factory runtime.PluginFactory) Option {
	return func(r runtime.Registry) error {
		return r.Register(name, factory)
	}
}

// NewSchedulerCommand creates a *cobra.Command object with default parameters
func NewSchedulerCommand(stopChan <-chan struct{}, registryOptions ...Option) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "karmada-scheduler",
		Long: `The karmada-scheduler is a control plane process which assigns resources to the clusters it manages.
The scheduler determines which clusters are valid placements for each resource in the scheduling queue according to
constraints and available resources. The scheduler then ranks each valid cluster and binds the resource to
the most suitable cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(opts, stopChan, registryOptions...); err != nil {
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
	opts.AddFlags(genericFlagSet)

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)
	cmd.AddCommand(sharedcommand.NewCmdVersion("karmada-scheduler"))

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

func run(opts *options.Options, stopChan <-chan struct{}, registryOptions ...Option) error {
	klog.Infof("karmada-scheduler version: %s", version.Get())
	go serveHealthzAndMetrics(net.JoinHostPort(opts.BindAddress, strconv.Itoa(opts.SecurePort)))

	profileflag.ListenAndServe(opts.ProfileOpts)

	restConfig, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	restConfig.QPS, restConfig.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	karmadaClient := karmadaclientset.NewForConfigOrDie(restConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(restConfig)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopChan
		cancel()
	}()
	outOfTreeRegistry := make(runtime.Registry)
	for _, option := range registryOptions {
		if err := option(outOfTreeRegistry); err != nil {
			return fmt.Errorf("register out of tree plugins error: %s", err)
		}
	}

	sched, err := scheduler.NewScheduler(dynamicClientSet, karmadaClient, kubeClientSet,
		scheduler.WithOutOfTreeRegistry(outOfTreeRegistry),
		scheduler.WithEnableSchedulerEstimator(opts.EnableSchedulerEstimator),
		scheduler.WithDisableSchedulerEstimatorInPullMode(opts.DisableSchedulerEstimatorInPullMode),
		scheduler.WithSchedulerEstimatorServicePrefix(opts.SchedulerEstimatorServicePrefix),
		scheduler.WithSchedulerEstimatorPort(opts.SchedulerEstimatorPort),
		scheduler.WithSchedulerEstimatorTimeout(opts.SchedulerEstimatorTimeout),
		scheduler.WithEnableEmptyWorkloadPropagation(opts.EnableEmptyWorkloadPropagation),
		scheduler.WithEnableSchedulerPlugin(opts.Plugins),
		scheduler.WithSchedulerName(opts.SchedulerName),
	)
	if err != nil {
		return fmt.Errorf("couldn't create scheduler: %w", err)
	}

	if !opts.LeaderElection.LeaderElect {
		sched.Run(ctx)
		return fmt.Errorf("scheduler exited")
	}

	leaderElectionClient, err := kubernetes.NewForConfig(rest.AddUserAgent(restConfig, "leader-election"))
	if err != nil {
		return err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("unable to get hostname: %v", err)
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id := hostname + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(opts.LeaderElection.ResourceLock,
		opts.LeaderElection.ResourceNamespace,
		opts.LeaderElection.ResourceName,
		leaderElectionClient.CoreV1(),
		leaderElectionClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
	if err != nil {
		return fmt.Errorf("couldn't create resource lock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: sched.Run,
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
	})

	return nil
}

func serveHealthzAndMetrics(address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/metrics", promhttp.HandlerFor(ctrlmetrics.Registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	}))

	httpServer := http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: ReadHeaderTimeout,
		WriteTimeout:      WriteTimeout,
		ReadTimeout:       ReadTimeout,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		klog.Errorf("Failed to serve healthz and metrics: %v", err)
		os.Exit(1)
	}
}
