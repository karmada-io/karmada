package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/karmada-scheduler/app/leaderelection"
	"github.com/karmada-io/karmada/cmd/karmada-scheduler/app/options"
)

// NewSchedulerCommand creates a *cobra.Command object with default parameters
func NewSchedulerCommand(stopChan <-chan struct{}) *cobra.Command {
	verFlag := false
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "karmada-scheduler",
		Long: `The karmada scheduler is a control plane process which assigns resources to clusters. 
The scheduler determines which clusters are valid placements for each resource in the scheduling queue 
according to constraints and available resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			if verFlag {
				os.Exit(0)
			}

			opts.Complete()
			if err := Run(opts, stopChan); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.Flags().BoolVar(&verFlag, "version", false, "Prints the version info.")

	return cmd
}

// Run runs the scheduler with options. This should never exit.
func Run(opts *options.Options, stopChan <-chan struct{}) error {
	logs.InitLogs()
	defer logs.FlushLogs()

	var err error
	// TODO(RainbowMango): need to change to shim kube-apiserver config.
	opts.KubeConfig, err = clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		panic(err)
	}

	elector, err := leaderelection.NewLeaderElector(opts, startScheduler)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-stopChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	elector.Run(ctx)

	klog.Errorf("lost lease")
	return errors.New("lost lease")
}

func startScheduler(opts *options.Options, stopChan <-chan struct{}) {
	// TODO(RainbowMango): Add implementation here.
	klog.Infof("starting scheduler.")
}
