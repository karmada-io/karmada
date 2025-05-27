/*
Copyright 2020 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/logs/json/register" // To enable JSON log format support
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/karmada-io/karmada/cmd/controller-manager/app"
)

func main() {
	ctx := controllerruntime.SetupSignalHandler()
	cmd := app.NewControllerManagerCommand(ctx)
	exitCode := cli.Run(cmd)
	// Ensure any buffered log entries are flushed
	logs.FlushLogs()
	os.Exit(exitCode)
}
