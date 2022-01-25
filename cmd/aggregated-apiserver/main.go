package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/aggregated-apiserver/app"
)

func main() {
	if err := runAggregatedApiserverCmd(); err != nil {
		os.Exit(1)
	}
}

func runAggregatedApiserverCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := apiserver.SetupSignalContext()
	if err := app.NewAggregatedApiserverCommand(ctx).Execute(); err != nil {
		return err
	}

	return nil
}
