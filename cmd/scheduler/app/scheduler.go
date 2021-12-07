package app

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler/app/options"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/scheduler"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewSchedulerCommand creates a *cobra.Command object with default parameters
func NewSchedulerCommand(stopChan <-chan struct{}) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "karmada-scheduler",
		Long: `The karmada scheduler binds resources to the clusters it manages.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(opts, stopChan); err != nil {
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
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "karmada-scheduler"))
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

func run(opts *options.Options, stopChan <-chan struct{}) error {
	klog.Infof("karmada-scheduler version: %s", version.Get())
	go serveHealthzAndMetrics(net.JoinHostPort(opts.BindAddress, strconv.Itoa(opts.SecurePort)))

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

	sched := scheduler.NewScheduler(dynamicClientSet, karmadaClient, kubeClientSet, opts)
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
	id := hostname + "_" + uuid.New().String()

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
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	http.Handle("/metrics", promhttp.Handler())

	klog.Fatal(http.ListenAndServe(address, nil))
}
