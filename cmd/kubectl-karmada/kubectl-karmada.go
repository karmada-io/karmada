package main

import (
	"os"

	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/pkg/karmadactl"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := karmadactl.NewKarmadaCtlCommand(os.Stdout, "karmada", "kubectl karmada").Execute(); err != nil {
		os.Exit(1)
	}
}
