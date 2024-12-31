/*
Copyright 2023 The Karmada Authors.

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

package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewCleanupCertTask init a task to cleanup certs
func NewCleanupCertTask(karmada *v1alpha1.Karmada) workflow.Task {
	workflowTasks := []workflow.Task{
		newCleanupCertSubTask("karmada", util.KarmadaCertSecretName),
		newCleanupCertSubTask("webhook", util.WebhookCertSecretName),
	}
	// Required only if local etcd is configured
	if karmada.Spec.Components.Etcd.Local != nil {
		cleanupEtcdCertTask := newCleanupCertSubTask("etcd", util.EtcdCertSecretName)
		workflowTasks = append(workflowTasks, cleanupEtcdCertTask)
	}
	return workflow.Task{
		Name:        "cleanup-cert",
		Run:         runCleanupCert,
		RunSubTasks: true,
		Tasks:       workflowTasks,
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

func newCleanupCertSubTask(owner string, secretNameFunc util.Namefunc) workflow.Task {
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
