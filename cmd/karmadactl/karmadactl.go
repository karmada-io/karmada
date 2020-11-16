package main

import (
	"fmt"
	"os"

	"k8s.io/component-base/logs"

	"github.com/huawei-cloudnative/karmada/pkg/karmadactl"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := karmadactl.NewKarmadaCtlCommand(os.Stdout).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
