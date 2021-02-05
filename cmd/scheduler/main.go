package main

import (
	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler/app"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopChan := apiserver.SetupSignalHandler()

	if err := app.NewSchedulerCommand(stopChan).Execute(); err != nil {
		klog.Fatal(err.Error())
	}
}
