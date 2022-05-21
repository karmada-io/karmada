package main

import (
	"os"

	"k8s.io/component-base/logs"

	"github.com/karmada-io/karmada/pkg/karmadactl"
)

func main() {
	if err := runKarmadaCtlCmd(); err != nil {
		os.Exit(1)
	}
}

func runKarmadaCtlCmd() error {
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := karmadactl.NewKarmadaCtlCommand("karmadactl", "karmadactl").Execute(); err != nil {
		return err
	}

	return nil
}
