package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app"
)

func main() {
	if err := runSchedulerEstimatorCmd(); err != nil {
		os.Exit(1)
	}
}

func runSchedulerEstimatorCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := apiserver.SetupSignalContext()
	if err := app.NewSchedulerEstimatorCommand(ctx).Execute(); err != nil {
		return err
	}

	return nil
}
