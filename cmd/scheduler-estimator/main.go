package main

import (
	"fmt"
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/scheduler-estimator/app"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := apiserver.SetupSignalContext()

	if err := app.NewSchedulerEstimatorCommand(ctx).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
