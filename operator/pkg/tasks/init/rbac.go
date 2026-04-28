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
