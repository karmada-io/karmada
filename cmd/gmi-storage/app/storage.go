/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/05/07 16:02:21
 Desc     :
*/

package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/kubernetes"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/gmi-storage/app/options"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/storage"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

func NewGmiStorageCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  names.KarmadaGmiStorageComponentName,
		Long: `The karmada-gmi-storage is a unified-storage-view tool of karmada`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := run(ctx, opts); err != nil {
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

	cmd.AddCommand(sharedcommand.NewCmdVersion(names.KarmadaGmiStorageComponentName))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	// cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	// sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)

	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-gmi-storage version: %s", version.Get())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	profileflag.ListenAndServe(opts.ProfileOpts)

	restConfig, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	restConfig.QPS, restConfig.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

	dynamicClientSet := dynamic.NewForConfigOrDie(restConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(restConfig)
	// it is necessary to initialize the storage first to prevent the events from failing,
	// which may lead to the inability to monitor the storage resources and thus miss the mounting.
	gmiStorage := storage.NewGMIStorage(ctx, kubeClientSet, dynamicClientSet)
	if err := gmiStorage.Init(ctx, dynamicClientSet); err != nil {
		return fmt.Errorf("failed to initialize storage: %s", err.Error())
	}
	ctx, cancel := context.WithCancel(ctx)
	// start event watcher for storage
	go func() {
		if err := gmiStorage.Watch(ctx); err != nil {
			klog.Errorf("failed to watch storage: %s", err.Error())
			return
		}
	}()

	<-signalChan
	// kstorage.Exit()
	cancel()
	return nil
}
