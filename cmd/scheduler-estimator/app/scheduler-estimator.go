package app

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app/options"
	"github.com/karmada-io/karmada/pkg/estimator/server"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

// NewSchedulerEstimatorCommand creates a *cobra.Command object with default parameters
func NewSchedulerEstimatorCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "karmada-scheduler-estimator",
		Long: `The karmada scheduler estimator runs an accurate scheduler estimator of a cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(ctx, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	opts.AddFlags(cmd.Flags())
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "karmada-scheduler-estimator"))
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-scheduler-estimator version: %s", version.Get())
	go serveHealthz(fmt.Sprintf("%s:%d", opts.BindAddress, opts.SecurePort))

	restConfig, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	restConfig.QPS, restConfig.Burst = opts.ClusterAPIQPS, opts.ClusterAPIBurst

	kubeClientSet := kubernetes.NewForConfigOrDie(restConfig)

	e := server.NewEstimatorServer(kubeClientSet, opts)
	if err = e.Start(ctx); err != nil {
		klog.Errorf("estimator server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

func serveHealthz(address string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	klog.Fatal(http.ListenAndServe(address, nil))
}
