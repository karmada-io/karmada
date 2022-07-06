package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration

	"github.com/karmada-io/karmada/examples/customresourceinterpreter/webhook/app"
)

func main() {
	ctx := apiserver.SetupSignalContext()
	cmd := app.NewWebhookCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
