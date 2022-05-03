package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/webhook/app"
)

func main() {
	if err := runWebhookCmd(); err != nil {
		os.Exit(1)
	}
}

func runWebhookCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := apiserver.SetupSignalContext()
	if err := app.NewWebhookCommand(ctx).Execute(); err != nil {
		return err
	}

	return nil
}
