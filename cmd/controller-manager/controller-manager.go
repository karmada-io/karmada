package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
	controllerruntime "sigs.k8s.io/controller-runtime"
	_ "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/karmada-io/karmada/cmd/controller-manager/app"
)

func main() {
	ctx := controllerruntime.SetupSignalHandler()
	cmd := app.NewControllerManagerCommand(ctx)
	code := cli.Run(cmd)
	os.Exit(code)
}
