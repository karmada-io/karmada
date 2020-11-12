package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/huawei-cloudnative/karmada/cmd/controller-manager/app/leaderelection"
	"github.com/huawei-cloudnative/karmada/cmd/controller-manager/app/options"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/membercluster"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
)

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(stopChan <-chan struct{}) *cobra.Command {
	verFlag := false
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "controller-manager",
		Long: `The controller manager runs a bunch of controllers`,
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

	cmd.Flags().BoolVar(&verFlag, "version", false, "Prints the version info of controller manager.")

	return cmd
}

// Run runs the controller-manager with options. This should never exit.
func Run(opts *options.Options, stopChan <-chan struct{}) error {
	logs.InitLogs()
	defer logs.FlushLogs()

	var err error
	// TODO(RainbowMango): need to change to shim kube-apiserver config.
	opts.KubeConfig, err = clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		panic(err)
	}

	if len(opts.HostNamespace) == 0 {
		// For in-cluster deployment set the namespace associated with the service account token
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			klog.Fatalf("An error occurred while attempting to discover the namespace from the service account: %v", err)
		}
		opts.HostNamespace = strings.TrimSpace(string(data))
	}

	// Validate if the namespace is configured
	if len(opts.HostNamespace) == 0 {
		klog.Fatalf("The namespace must be specified")
	}

	elector, err := leaderelection.NewLeaderElector(opts, startControllers)
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

func startControllers(opts *options.Options, stopChan <-chan struct{}) {
	controllerConfig := &util.ControllerConfig{
		HeadClusterConfig: opts.KubeConfig,
	}

	if err := membercluster.StartMemberClusterController(controllerConfig, stopChan); err != nil {
		klog.Fatalf("Failed to start member cluster controller. error: %v", err)
	}
}
