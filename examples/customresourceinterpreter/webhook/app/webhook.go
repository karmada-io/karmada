/*
Copyright 2021 The Karmada Authors.

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
	"net/http"
	"os"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/karmada-io/karmada/examples/customresourceinterpreter/webhook/app/options"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
	"github.com/karmada-io/karmada/pkg/webhook/interpreter"
)

// NewWebhookCommand creates a *cobra.Command object with default parameters
func NewWebhookCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "karmada-interpreter-webhook-example",
		Run: func(_ *cobra.Command, _ []string) {
			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				fmt.Fprintf(os.Stderr, "configuration is not valid: %v\n", errs.ToAggregate())
				os.Exit(1)
			}

			if err := Run(ctx, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(genericFlagSet)

	// Set klog flags
	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)

	cmd.AddCommand(sharedcommand.NewCmdVersion("karmada-interpreter-webhook-example"))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

// Run runs the webhook server with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}

	hookManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger: klog.Background(),
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    opts.BindAddress,
			Port:    opts.SecurePort,
			CertDir: opts.CertDir,
		}),
		LeaderElection: false,
	})
	if err != nil {
		klog.Errorf("Failed to build webhook server: %v", err)
		return err
	}

	klog.Info("Registering webhooks to the webhook server")
	hookServer := hookManager.GetWebhookServer()
	hookServer.Register("/interpreter-workload", interpreter.NewWebhook(&workloadInterpreter{}, interpreter.NewDecoder(gclient.NewSchema())))
	hookServer.WebhookMux().Handle("/readyz/", http.StripPrefix("/readyz/", &healthz.Handler{}))

	// blocks until the context is done.
	if err := hookManager.Start(ctx); err != nil {
		klog.Errorf("Webhook server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}
