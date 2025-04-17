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

	cmds, err := generateCMDs(module)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, cmd := range cmds {
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
}

func generateCMDs(module string) ([]*cobra.Command, error) {
	var cmds []*cobra.Command
	switch module {
	case names.KarmadaControllerManagerComponentName:
		// generate docs for karmada-controller-manager
		cmds = append(cmds, cmapp.NewControllerManagerCommand(context.TODO()))
	case names.KarmadaSchedulerComponentName:
		// generate docs for karmada-scheduler
		cmds = append(cmds, schapp.NewSchedulerCommand(context.TODO()))
	case names.KarmadaAgentComponentName:
		// generate docs for karmada-agent
		cmds = append(cmds, agentapp.NewAgentCommand(context.TODO()))
	case names.KarmadaAggregatedAPIServerComponentName:
		// generate docs for karmada-aggregated-apiserver
		cmds = append(cmds, aaapp.NewAggregatedApiserverCommand(context.TODO()))
	case names.KarmadaDeschedulerComponentName:
		// generate docs for karmada-descheduler
		cmds = append(cmds, deschapp.NewDeschedulerCommand(context.TODO()))
	case names.KarmadaSearchComponentName:
		// generate docs for karmada-search
		cmds = append(cmds, searchapp.NewKarmadaSearchCommand(context.TODO()))
	case names.KarmadaSchedulerEstimatorComponentName:
		// generate docs for karmada-scheduler-estimator
		cmds = append(cmds, estiapp.NewSchedulerEstimatorCommand(context.TODO()))
	case names.KarmadaWebhookComponentName:
		// generate docs for karmada-webhook
		cmds = append(cmds, webhookapp.NewWebhookCommand(context.TODO()))
	case names.KarmadaMetricsAdapterComponentName:
		// generate docs for karmada-metrics-adapter
		cmds = append(cmds, adapterapp.NewMetricsAdapterCommand(context.TODO()))
	case "all":
		cmds = append(cmds, cmapp.NewControllerManagerCommand(context.TODO()))
		cmds = append(cmds, schapp.NewSchedulerCommand(context.TODO()))
		cmds = append(cmds, agentapp.NewAgentCommand(context.TODO()))
		cmds = append(cmds, aaapp.NewAggregatedApiserverCommand(context.TODO()))
		cmds = append(cmds, deschapp.NewDeschedulerCommand(context.TODO()))
		cmds = append(cmds, searchapp.NewKarmadaSearchCommand(context.TODO()))
		cmds = append(cmds, estiapp.NewSchedulerEstimatorCommand(context.TODO()))
		cmds = append(cmds, webhookapp.NewWebhookCommand(context.TODO()))
		cmds = append(cmds, adapterapp.NewMetricsAdapterCommand(context.TODO()))
	default:
		return nil, fmt.Errorf("module %s is not supported", module)
	}

	return cmds, nil
}
