package main

import (
	"os"

	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration

	"github.com/karmada-io/karmada/pkg/karmadactl"
)

func main() {
	cmd := karmadactl.NewKarmadaCtlCommand("karmadactl", "karmadactl")
	code := cli.Run(cmd)
	os.Exit(code)
}
