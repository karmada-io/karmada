package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/scheduler/app"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopChan := apiserver.SetupSignalHandler()

	if err := app.NewSchedulerCommand(stopChan).Execute(); err != nil {
		os.Exit(1)
	}
}
