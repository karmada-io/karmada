package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
	controllerruntime "sigs.k8s.io/controller-runtime"
	_ "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/karmada-io/karmada/operator/cmd/operator/app"
)

func main() {
	ctx := controllerruntime.SetupSignalHandler()
	command := app.NewOperatorCommand(ctx)
	code := cli.Run(command)
	os.Exit(code)
}
