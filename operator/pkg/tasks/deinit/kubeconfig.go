package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewCleanupKubeconfigTask init a task to cleanup kubeconfig
func NewCleanupKubeconfigTask() workflow.Task {
	return workflow.Task{
		Name: "cleanup-kubeconfig",
		Run:  runCleanupKubeconfig,
	}
}

func runCleanupKubeconfig(r workflow.RunData) error {
	data, ok := r.(DeInitData)
	if !ok {
		return errors.New("cleanup-kubeconfig task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[cleanup-kubeconfig] Running cleanup-kubeconfig task", "karmada", klog.KObj(data))

	err := apiclient.DeleteSecretIfHasLabels(
		data.RemoteClient(),
		util.AdminKubeconfigSecretName(data.GetName()),
		data.GetNamespace(),
		constants.KarmadaOperatorLabel,
	)
	if err != nil {
		return fmt.Errorf("failed to cleanup karmada kubeconfig, err: %w", err)
	}

	return nil
}
