package main

import (
	"k8s.io/component-base/cli"
	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/karmada-io/karmada/pkg/karmadactl"
)

func main() {
	cmd := karmadactl.NewKarmadaCtlCommand("karmadactl", "karmadactl")
	if err := cli.RunNoErrOutput(cmd); err != nil {
		util.CheckErr(err)
	}
}
