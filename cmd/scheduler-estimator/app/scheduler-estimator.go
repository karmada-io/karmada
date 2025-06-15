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
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app/options"
	"github.com/karmada-io/karmada/pkg/estimator/server"
	"github.com/karmada-io/karmada/pkg/features"
	versionmetrics "github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli"
	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"github.com/karmada-io/karmada/pkg/sharedcli/profileflag"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
	"github.com/karmada-io/karmada/pkg/version/sharedcommand"
)

const (
	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	// References:
	// - https://en.wikipedia.org/wiki/Slowloris_(computer_security)
	ReadHeaderTimeout = 32 * time.Second
	// WriteTimeout is the amount of time allowed to write the
	// request data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	WriteTimeout = 5 * time.Minute
	// ReadTimeout is the amount of time allowed to read
	// response data.
	// HTTP timeouts are necessary to expire inactive connections
	// and failing to do so might make the application vulnerable
	// to attacks like slowloris which work by sending data very slow,
	// which in case of no timeout will keep the connection active
	// eventually leading to a denial-of-service (DoS) attack.
	ReadTimeout = 5 * time.Minute
)

// NewSchedulerEstimatorCommand creates a *cobra.Command object with default parameters
func NewSchedulerEstimatorCommand(ctx context.Context) *cobra.Command {
	logConfig := logsv1.NewLoggingConfiguration()
	fss := cliflag.NamedFlagSets{}

	logsFlagSet := fss.FlagSet("logs")
	logs.AddFlags(logsFlagSet, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logConfig, logsFlagSet)
	klogflag.Add(logsFlagSet)

	genericFlagSet := fss.FlagSet("generic")
	opts := options.NewOptions()
	opts.AddFlags(genericFlagSet)

	cmd := &cobra.Command{
		Use: names.KarmadaSchedulerEstimatorComponentName,
		Long: `The karmada-scheduler-estimator runs an accurate scheduler estimator of a cluster. It
provides the scheduler with more accurate cluster resource information.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := logsv1.ValidateAndApply(logConfig, features.FeatureGate); err != nil {
				return err
			}
			logs.InitLogs()
			return nil
		},
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
	}

	cmd.AddCommand(sharedcommand.NewCmdVersion(names.KarmadaSchedulerEstimatorComponentName))
	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

func run(ctx context.Context, opts *options.Options) error {
	klog.Infof("karmada-scheduler-estimator version: %s", version.Get())

	ctrlmetrics.Registry.MustRegister(versionmetrics.NewBuildInfoCollector())

	serveHealthzAndMetrics(opts.HealthProbeBindAddress, opts.MetricsBindAddress)

	profileflag.ListenAndServe(opts.ProfileOpts)

	restConfig, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	restConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(opts.ClusterAPIQPS, opts.ClusterAPIBurst)

	kubeClient := kubernetes.NewForConfigOrDie(restConfig)
	dynamicClient := dynamic.NewForConfigOrDie(restConfig)
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(restConfig)

	e, err := server.NewEstimatorServer(ctx, kubeClient, dynamicClient, discoveryClient, opts)
	if err != nil {
		klog.Errorf("Fail to create estimator server: %v", err)
		return err
	}

	if err = e.Start(ctx); err != nil {
		klog.Errorf("Estimator server exits unexpectedly: %v", err)
		return err
	}

	// never reach here
	return nil
}

func serveHealthzAndMetrics(healthProbeBindAddress, metricsBindAddress string) {
	if healthProbeBindAddress == metricsBindAddress {
		if healthProbeBindAddress != "0" {
			go serveCombined(healthProbeBindAddress)
		}
	} else {
		if healthProbeBindAddress != "0" {
			go serveHealthz(healthProbeBindAddress)
		}
		if metricsBindAddress != "0" {
			go serveMetrics(metricsBindAddress)
		}
	}
}

func serveCombined(address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.Handle("/metrics", metricsHandler())

	serveHTTP(address, mux, "healthz and metrics")
}

func serveHealthz(address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	serveHTTP(address, mux, "healthz")
}

func serveMetrics(address string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler())
	serveHTTP(address, mux, "metrics")
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func metricsHandler() http.Handler {
	return promhttp.HandlerFor(ctrlmetrics.Registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
}

func serveHTTP(address string, handler http.Handler, name string) {
	httpServer := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: ReadHeaderTimeout,
		WriteTimeout:      WriteTimeout,
		ReadTimeout:       ReadTimeout,
	}

	klog.Infof("Starting %s server on %s", name, address)
	if err := httpServer.ListenAndServe(); err != nil {
		klog.Errorf("Failed to serve %s on %s: %v", name, address, err)
		os.Exit(1)
	}
}
