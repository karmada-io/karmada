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

// NewCleanupCertTask init a task to cleanup certs
func NewCleanupCertTask() workflow.Task {
	return workflow.Task{
		Name:        "cleanup-cert",
		Run:         runCleanupCert,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newClenupCertSubTask("karmada", util.KarmadaCertSecretName),
			newClenupCertSubTask("etcd", util.EtcdCertSecretName),
			newClenupCertSubTask("webhook", util.WebhookCertSecretName),
		},
	}
}

func runCleanupCert(r workflow.RunData) error {
	data, ok := r.(DeInitData)
	if !ok {
		return errors.New("cleanup-cert task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[cleanup-cert] Running cleanup-cert task", "karmada", klog.KObj(data))
	return nil
}

func newClenupCertSubTask(owner string, secretNameFunc util.Namefunc) workflow.Task {
	return workflow.Task{
		Name: fmt.Sprintf("cleanup-%s-cert", owner),
		Run:  runCleanupCertSubTask(owner, secretNameFunc),
	}
}

func runCleanupCertSubTask(owner string, secretNameFunc util.Namefunc) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(DeInitData)
		if !ok {
			return fmt.Errorf("cleanup-%s-cert task invoked with an invalid data struct", owner)
		}

		err := apiclient.DeleteSecretIfHasLabels(
			data.RemoteClient(),
			secretNameFunc(data.GetName()),
			data.GetNamespace(),
			constants.KarmadaOperatorLabel,
		)
		if err != nil {
			return fmt.Errorf("failed to cleanup %s certs, err: %w", owner, err)
		}

		return nil
	}
}
