/*
Copyright 2024 The Karmada Authors.

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
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/term"
	ctrlmgrhealthz "k8s.io/controller-manager/pkg/healthz"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/service-name-resolution-detector-example/app/options"
	"github.com/karmada-io/karmada/cmd/service-name-resolution-detector-example/names"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/servicenameresolutiondetector/coredns"
	"github.com/karmada-io/karmada/pkg/sharedcli"
)

// NewDetector creates a *cobra.Command object with default parameters.
func NewDetector(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "service-name-resolution-detector-example",
		Long: `The service-name-resolution-detector-example is an agent deployed in member clusters. It can periodically detect health 
condition of components in member cluster(such as coredns), and sync conditions to Karmada control plane.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := runCmd(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}

	fss := opts.Flags(KnownDetectors())
	fs := cmd.Flags()
	for _, f := range fss.FlagSets {
		fs.AddFlagSet(f)
	}

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	sharedcli.SetUsageAndHelpFunc(cmd, fss, cols)
	return cmd
}

func runCmd(ctx context.Context, opts *options.Options) error {
	logger := klog.FromContext(ctx)
	logger.Info("start cluster problem detector")

	var checks []healthz.HealthChecker
	var electionChecker *leaderelection.HealthzAdaptor
	if opts.Generic.LeaderElection.LeaderElect {
		electionChecker = leaderelection.NewLeaderHealthzAdaptor(20 * time.Second)
		checks = append(checks, electionChecker)
	}

	detectorCtx, err := createDetectorContext(opts)
	if err != nil {
		logger.Error(err, "Error building detector context")
		return err
	}

	for name, initFn := range NewDetectorInitializers() {
		if !detectorCtx.IsDetectorEnabled(name) {
			logger.Info("Warning: detector is disabled", "detector", name)
			continue
		}

		logger.V(1).Info("Starting detector", "detector", name)
		started, err := initFn(klog.NewContext(ctx, klog.LoggerWithName(logger, name)), detectorCtx)
		if err != nil {
			logger.Error(err, "Error starting detector", "detector", name)
			return err
		}
		if !started {
			logger.Info("Warning: skipping detector", "detector", name)
			continue
		}
		checks = append(checks, ctrlmgrhealthz.NamedPingChecker(name))
		logger.Info("Started detector", "detector", name)
	}

	detectorCtx.sharedInformers.Start(ctx.Done())
	defer detectorCtx.sharedInformers.Shutdown()
	for t, s := range detectorCtx.sharedInformers.WaitForCacheSync(ctx.Done()) {
		if !s {
			logger.Error(err, "Error starting informer", "informer", t)
			return fmt.Errorf("informer %v not start", t)
		}
	}

	go startHealthzServer(ctx, ctrlmgrhealthz.NewMutableHealthzHandler(checks...), opts.Generic.BindAddress, opts.Generic.HealthzPort)

	<-ctx.Done()
	return nil
}

var detectorsDisabledByDefault = sets.NewString()

// KnownDetectors return names of all detectors.
func KnownDetectors() []string {
	return sets.StringKeySet(NewDetectorInitializers()).List()
}

type detectorContext struct {
	controlPlaneClient karmada.Interface
	memberClient       kubernetes.Interface
	sharedInformers    informers.SharedInformerFactory

	lec         componentbaseconfig.LeaderElectionConfiguration
	hostName    string
	clusterName string
	detectors   []string

	corednsConfig *coredns.Config
}

func createDetectorContext(opts *options.Options) (detectorContext, error) {
	controlPlaneClient := karmada.NewForConfigOrDie(opts.Generic.KarmadaConfig)
	memberClient := kubernetes.NewForConfigOrDie(opts.Generic.MemberClusterConfig)
	sharedInformers := informers.NewSharedInformerFactory(memberClient, 10*time.Minute)

	detectorCtx := detectorContext{
		controlPlaneClient: controlPlaneClient,
		memberClient:       memberClient,
		sharedInformers:    sharedInformers,
		lec:                opts.Generic.LeaderElection,
		hostName:           opts.Generic.Hostname,
		clusterName:        opts.Generic.ClusterName,
		detectors:          opts.Generic.Detectors,
		corednsConfig: &coredns.Config{
			Period:           opts.Coredns.Period,
			SuccessThreshold: opts.Coredns.SuccessThreshold,
			FailureThreshold: opts.Coredns.FailureThreshold,
			StaleThreshold:   opts.Coredns.StaleThreshold,
		},
	}
	return detectorCtx, nil
}

func (c detectorContext) IsDetectorEnabled(name string) bool {
	hasStar := false
	for _, detector := range c.detectors {
		if detector == name {
			return true
		}
		if detector == "-"+name {
			return false
		}
		if detector == "*" {
			hasStar = true
		}
	}
	// if we get here, there was no explicit choice
	if !hasStar {
		// nothing on by default
		return false
	}
	return !detectorsDisabledByDefault.Has(name)
}

// InitFunc inits a detector.
type InitFunc func(ctx context.Context, detectorContext detectorContext) (started bool, err error)

// NewDetectorInitializers registers init functions for all detectors.
func NewDetectorInitializers() map[string]InitFunc {
	detectors := map[string]InitFunc{}

	register := func(name string, fn InitFunc) {
		if _, ok := detectors[name]; !ok {
			detectors[name] = fn
		} else {
			panic(fmt.Sprintf("detector name %q was registered twice", name))
		}
	}

	register(names.CorednsDetector, startCorednsDetector)

	return detectors
}

func startHealthzServer(ctx context.Context, handler *ctrlmgrhealthz.MutableHealthzHandler, addr string, port int) {
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", addr, port),
		Handler:           handler,
		MaxHeaderBytes:    1 << 20,
		IdleTimeout:       90 * time.Second,
		ReadHeaderTimeout: 32 * time.Second,
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			klog.FromContext(ctx).Error(err, "Error starting healthz server")
		}
	}()
}
