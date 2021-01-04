package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/karmada-io/karmada/cmd/scheduler/app/options"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/scheduler"
)

// NewSchedulerCommand creates a *cobra.Command object with default parameters
func NewSchedulerCommand(stopChan <-chan struct{}) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "scheduler",
		Long: `The karmarda scheduler binds resources to the clusters it manages.`,
		Run: func(cmd *cobra.Command, args []string) {
			opts.Complete()
			if err := run(opts, stopChan); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	opts.AddFlags(cmd.Flags())
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

func run(opts *options.Options, stopChan <-chan struct{}) error {
	resetConfig, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}

	// TODO: add leader election

	dynamicClientSet := dynamic.NewForConfigOrDie(resetConfig)
	karmadaClient := karmadaclientset.NewForConfigOrDie(resetConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(resetConfig)

	sched := scheduler.NewScheduler(dynamicClientSet, karmadaClient, kubeClientSet)
	sched.Run(stopChan)
	return nil
}
