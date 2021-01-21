package main

import (
	"fmt"
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/webhook/app"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	stopChan := apiserver.SetupSignalHandler()

	if err := app.NewWebhookCommand(stopChan).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
