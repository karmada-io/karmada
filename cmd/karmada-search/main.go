package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/cmd/karmada-search/app"
)

func main() {
	if err := runKarmadaSearchCmd(); err != nil {
		os.Exit(1)
	}
}

func runKarmadaSearchCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := apiserver.SetupSignalContext()
	if err := app.NewKarmadaSearchCommand(ctx).Execute(); err != nil {
		return err
	}

	return nil
}
