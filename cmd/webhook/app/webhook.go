package app

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/karmada-io/karmada/cmd/webhook/app/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
	"github.com/karmada-io/karmada/pkg/webhook/cluster"
	"github.com/karmada-io/karmada/pkg/webhook/clusterpropagationpolicy"
	"github.com/karmada-io/karmada/pkg/webhook/overridepolicy"
	"github.com/karmada-io/karmada/pkg/webhook/propagationpolicy"
	"github.com/karmada-io/karmada/pkg/webhook/work"
)

// NewWebhookCommand creates a *cobra.Command object with default parameters
func NewWebhookCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "karmada-webhook",
		Long: `Start a karmada webhook server`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := Run(ctx, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	cmd.AddCommand(sharedcommand.NewCmdVersion(os.Stdout, "karmada-webhook"))
	opts.AddFlags(cmd.Flags())

	return cmd
}

// Run runs the webhook server with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-webhook version: %s", version.Get())
	config, err := controllerruntime.GetConfig()
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubeAPIQPS, opts.KubeAPIBurst

	hookManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme:         gclient.NewSchema(),
		Host:           opts.BindAddress,
		Port:           opts.SecurePort,
		CertDir:        opts.CertDir,
		LeaderElection: false,
	})
	if err != nil {
		klog.Errorf("failed to build webhook server: %v", err)
		return err
	}

	klog.Info("registering webhooks to the webhook server")
	hookServer := hookManager.GetWebhookServer()
	hookServer.Register("/validate-cluster", &webhook.Admission{Handler: &cluster.ValidatingAdmission{}})
	hookServer.Register("/mutate-propagationpolicy", &webhook.Admission{Handler: &propagationpolicy.MutatingAdmission{}})
	hookServer.Register("/validate-propagationpolicy", &webhook.Admission{Handler: &propagationpolicy.ValidatingAdmission{}})
	hookServer.Register("/mutate-clusterpropagationpolicy", &webhook.Admission{Handler: &clusterpropagationpolicy.MutatingAdmission{}})
	hookServer.Register("/validate-clusterpropagationpolicy", &webhook.Admission{Handler: &clusterpropagationpolicy.ValidatingAdmission{}})
	hookServer.Register("/mutate-overridepolicy", &webhook.Admission{Handler: &overridepolicy.MutatingAdmission{}})
	hookServer.Register("/mutate-work", &webhook.Admission{Handler: &work.MutatingAdmission{}})
	hookServer.WebhookMux.Handle("/readyz/", http.StripPrefix("/readyz/", &healthz.Handler{}))

	// blocks until the context is done.
	if err := hookManager.Start(ctx); err != nil {
		klog.Errorf("webhook server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}
