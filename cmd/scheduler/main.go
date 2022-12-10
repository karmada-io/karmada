package main

import (
	"os"

	// Note that Kubernetes registers workqueue metrics to default prometheus Registry. And the registry will be
	// initialized by the package 'k8s.io/apiserver/pkg/server'.
	// See https://github.com/kubernetes/kubernetes/blob/release-1.26/staging/src/k8s.io/component-base/metrics/prometheus/workqueue/metrics.go#L25-L26
	// But the controller-runtime registers workqueue metrics to its own Registry instead of default prometheus Registry.
	// See https://github.com/kubernetes-sigs/controller-runtime/blob/release-0.14/pkg/metrics/workqueue.go#L24-L26
	// However, global workqueue metrics factory will be only initialized once.
	// See https://github.com/kubernetes/kubernetes/blob/release-1.26/staging/src/k8s.io/client-go/util/workqueue/metrics.go#L257-L261
	// So this package should be initialized before 'k8s.io/apiserver/pkg/server', thus the internal registry of
	// controller-runtime could be set first.
	_ "sigs.k8s.io/controller-runtime/pkg/metrics"

	apiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"
	_ "k8s.io/component-base/logs/json/register" // for JSON log format registration

	"github.com/karmada-io/karmada/cmd/scheduler/app"
)

func main() {
	stopChan := apiserver.SetupSignalHandler()
	command := app.NewSchedulerCommand(stopChan)
	code := cli.Run(command)
	os.Exit(code)
}
