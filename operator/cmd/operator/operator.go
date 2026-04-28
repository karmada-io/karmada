/*
Copyright 2021 The Karmada Authors.

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
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/karmada-io/karmada/operator/cmd/operator/app"
)

func main() {
	ctx := controllerruntime.SetupSignalHandler()
	// Starting from version 0.15.0, controller-runtime expects its consumers to set a logger through log.SetLogger.
	// If SetLogger is not called within the first 30 seconds of a binaries lifetime, it will get
	// set to a NullLogSink and report an error. Here's to silence the "log.SetLogger(...) was never called; logs will not be displayed" error
	// by setting a logger through log.SetLogger.
	// More info refer to: https://github.com/karmada-io/karmada/pull/4885.
	command := app.NewOperatorCommand(ctx)
	exitCode := cli.Run(command)
	// Ensure any buffered log entries are flushed
	logs.FlushLogs()
	os.Exit(exitCode)
}
