package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app/options"
	"github.com/karmada-io/karmada/pkg/estimator/server"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewSchedulerEstimatorCommand creates a *cobra.Command object with default parameters
func NewSchedulerEstimatorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "karmada-scheduler-estimator",
		Long: `The karmada scheduler estimator runs an accurate scheduler estimator of a cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(ctx, opts); err != nil {
				return err
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

	cmd.AddCommand(sharedcommand.NewCmdVersion("karmada-scheduler-estimator"))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-scheduler-estimator version: %s", version.Get())
	go serveHealthzAndMetrics(net.JoinHostPort(opts.BindAddress, strconv.Itoa(opts.SecurePort)))

	restConfig, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	restConfig.QPS, restConfig.Burst = opts.ClusterAPIQPS, opts.ClusterAPIBurst

	kubeClient := kubernetes.NewForConfigOrDie(restConfig)
	dynamicClient := dynamic.NewForConfigOrDie(restConfig)
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(restConfig)

	e := server.NewEstimatorServer(kubeClient, dynamicClient, discoveryClient, opts, ctx.Done())
	if err = e.Start(ctx); err != nil {
		klog.Errorf("estimator server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

func serveHealthzAndMetrics(address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/metrics", promhttp.Handler())

	klog.Fatal(http.ListenAndServe(address, mux))
}
