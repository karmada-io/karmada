package main

import (
	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/agent/app"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := apiserver.SetupSignalContext()

	if err := app.NewAgentCommand(ctx).Execute(); err != nil {
		klog.Fatal(err.Error())
	}
}
