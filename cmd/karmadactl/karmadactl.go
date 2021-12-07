package main

import (
	"os"

	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/pkg/karmadactl"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := karmadactl.NewKarmadaCtlCommand(os.Stdout, "karmadactl", "karmadactl").Execute(); err != nil {
		os.Exit(1)
	}
}
