package app

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/karmada-io/karmada/cmd/webhook/app/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/webhook/overridepolicy"
)

// NewWebhookCommand creates a *cobra.Command object with default parameters
func NewWebhookCommand(stopChan <-chan struct{}) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "webhook",
		Long: `Start a webhook server`,
		Run: func(cmd *cobra.Command, args []string) {
			opts.Complete()
			if err := Run(opts, stopChan); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

// Run runs the webhook server with options. This should never exit.
func Run(opts *options.Options, stopChan <-chan struct{}) error {
	logs.InitLogs()
	defer logs.FlushLogs()

	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	hookManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:           gclient.NewSchema(),
		Host:             "0.0.0.0",
		Port:             8443,
		CertDir:          "/var/serving-cert",
		LeaderElection:   false,
		LeaderElectionID: "webhook.karmada.io",
	})
	if err != nil {
		klog.Errorf("failed to build webhook server: %v", err)
		return err
	}

	klog.Info("registering webhooks to the webhook server")
	hookServer := hookManager.GetWebhookServer()
	hookServer.Register("/mutate-overridepolicy", &webhook.Admission{Handler: &overridepolicy.MutatingAdmission{}})
	hookServer.WebhookMux.Handle("/readyz/", http.StripPrefix("/readyz/", &healthz.Handler{}))

	// blocks until the stop channel is closed.
	if err := hookManager.Start(stopChan); err != nil {
		klog.Errorf("webhook server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}
