package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/karmada-io/karmada/cmd/aggregated-apiserver/app"
)

func main() {
	ctx := controllerruntime.SetupSignalHandler()
	cmd := app.NewAggregatedApiserverCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
