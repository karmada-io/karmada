package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	agentapp "github.com/karmada-io/karmada/cmd/agent/app"
	aaapp "github.com/karmada-io/karmada/cmd/aggregated-apiserver/app"
	cmapp "github.com/karmada-io/karmada/cmd/controller-manager/app"
	deschapp "github.com/karmada-io/karmada/cmd/descheduler/app"
	searchapp "github.com/karmada-io/karmada/cmd/karmada-search/app"
	adapterapp "github.com/karmada-io/karmada/cmd/metrics-adapter/app"
	estiapp "github.com/karmada-io/karmada/cmd/scheduler-estimator/app"
	schapp "github.com/karmada-io/karmada/cmd/scheduler/app"
	webhookapp "github.com/karmada-io/karmada/cmd/webhook/app"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

func main() {
	// use os.Args instead of "flags" because "flags" will mess up the man pages!
	path := ""
	module := ""
	if len(os.Args) == 3 {
		path = os.Args[1]
		module = os.Args[2]
	} else {
		fmt.Fprintf(os.Stderr, "usage: %s [output directory] [module] \n", os.Args[0])
		os.Exit(1)
	}

	outDir, err := lifted.OutDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get output directory: %v\n", err)
		os.Exit(1)
	}

	var cmd *cobra.Command

	switch module {
	case "karmada-controller-manager":
		// generate docs for karmada-controller-manager
		cmd = cmapp.NewControllerManagerCommand(context.TODO())
	case "karmada-scheduler":
		// generate docs for karmada-scheduler
		cmd = schapp.NewSchedulerCommand(nil)
	case "karmada-agent":
		// generate docs for karmada-agent
		cmd = agentapp.NewAgentCommand(context.TODO())
	case "karmada-aggregated-apiserver":
		// generate docs for karmada-aggregated-apiserver
		cmd = aaapp.NewAggregatedApiserverCommand(context.TODO())
	case "karmada-descheduler":
		// generate docs for karmada-descheduler
		cmd = deschapp.NewDeschedulerCommand(nil)
	case "karmada-search":
		// generate docs for karmada-search
		cmd = searchapp.NewKarmadaSearchCommand(context.TODO())
	case "karmada-scheduler-estimator":
		// generate docs for karmada-scheduler-estimator
		cmd = estiapp.NewSchedulerEstimatorCommand(context.TODO())
	case "karmada-webhook":
		// generate docs for karmada-webhook
		cmd = webhookapp.NewWebhookCommand(context.TODO())
	case "karmada-metrics-adapter":
		// generate docs for karmada-metrics-adapter
		cmd = adapterapp.NewMetricsAdapterCommand(context.TODO())
	default:
		fmt.Fprintf(os.Stderr, "Module %s is not supported", module)
		os.Exit(1)
	}

	cmd.DisableAutoGenTag = true
	err = doc.GenMarkdownTree(cmd, outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate docs: %v\n", err)
		os.Exit(1)
	}
	err = MarkdownPostProcessing(cmd, outDir, cleanupForInclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to cleanup docs: %v\n", err)
		os.Exit(1)
	}
}
