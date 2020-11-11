package main

import (
	"fmt"
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/huawei-cloudnative/karmada/cmd/controller-manager/app"
)

// Controller-manager main.
func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopChan := apiserver.SetupSignalHandler()

	if err := app.NewControllerManagerCommand(stopChan).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
