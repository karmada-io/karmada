package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/scheduler/app"
)

func main() {
	if err := runSchedulerCmd(); err != nil {
		os.Exit(1)
	}
}

func runSchedulerCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopChan := apiserver.SetupSignalHandler()
	command := app.NewSchedulerCommand(stopChan)
	if err := command.Execute(); err != nil {
		return err
	}

	return nil
}
