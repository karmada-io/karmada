package tasks

import (
	"errors"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/karmadaresource/rbac"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewRBACTask init a RBAC task, it will create clusterrole for view/edit karmada resources
func NewRBACTask() workflow.Task {
	return workflow.Task{
		Name: "rbac",
		Run:  runRBAC,
	}
}

func runRBAC(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("RBAC task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[RBAC] Running rbac task", "karmada", klog.KObj(data))

	return rbac.EnsureKarmadaRBAC(data.KarmadaClient())
}
