package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/descheduler/app"
)

func main() {
	if err := runDeschedulerCmd(); err != nil {
		os.Exit(1)
	}
}

func runDeschedulerCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopChan := apiserver.SetupSignalHandler()
	command := app.NewDeschedulerCommand(stopChan)
	if err := command.Execute(); err != nil {
		return err
	}

	return nil
}
