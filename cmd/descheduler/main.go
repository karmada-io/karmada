package main

import (
	"os"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration

	"github.com/karmada-io/karmada/cmd/descheduler/app"
)

func main() {
	stopChan := apiserver.SetupSignalHandler()
	command := app.NewDeschedulerCommand(stopChan)
	code := cli.Run(command)
	os.Exit(code)
}
