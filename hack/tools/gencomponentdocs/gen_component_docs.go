/*
Copyright 2022 The Karmada Authors.

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
	"github.com/karmada-io/karmada/pkg/util/names"
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
	case names.KarmadaControllerManagerComponentName:
		// generate docs for karmada-controller-manager
		cmd = cmapp.NewControllerManagerCommand(context.TODO())
	case names.KarmadaSchedulerComponentName:
		// generate docs for karmada-scheduler
		cmd = schapp.NewSchedulerCommand(nil)
	case names.KarmadaAgentComponentName:
		// generate docs for karmada-agent
		cmd = agentapp.NewAgentCommand(context.TODO())
	case names.KarmadaAggregatedAPIServerComponentName:
		// generate docs for karmada-aggregated-apiserver
		cmd = aaapp.NewAggregatedApiserverCommand(context.TODO())
	case names.KarmadaDeschedulerComponentName:
		// generate docs for karmada-descheduler
		cmd = deschapp.NewDeschedulerCommand(nil)
	case names.KarmadaSearchComponentName:
		// generate docs for karmada-search
		cmd = searchapp.NewKarmadaSearchCommand(context.TODO())
	case names.KarmadaSchedulerEstimatorComponentName:
		// generate docs for karmada-scheduler-estimator
		cmd = estiapp.NewSchedulerEstimatorCommand(context.TODO())
	case names.KarmadaWebhookComponentName:
		// generate docs for karmada-webhook
		cmd = webhookapp.NewWebhookCommand(context.TODO())
	case names.KarmadaMetricsAdapterComponentName:
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
