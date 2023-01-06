package tasks

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewNamespaceTask init a task to create namespace
func NewNamespaceTask() workflow.Task {
	return workflow.Task{
		Name: "Namespace",
		Run:  runNamespace,
	}
}

func runNamespace(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("namespace task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[namespace] Running namespace task", "karmada", klog.KObj(data))

	err := apiclient.CreateNamespace(data.RemoteClient(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: data.GetNamespace(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create namespace for %s, err: %w", data.GetName(), err)
	}

	return nil
}
