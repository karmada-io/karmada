/*
Copyright 2024 The Karmada Authors.

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

package karmada

import (
	"fmt"
	"reflect"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

var (
	// clientFactory creates a new Kubernetes clientset from the provided kubeconfig.
	clientFactory = func(kubeconfig *rest.Config) (clientset.Interface, error) {
		return clientset.NewForConfig(kubeconfig)
	}

	// buildClientFromSecretRefFactory constructs a Kubernetes clientset using a LocalSecretReference.
	buildClientFromSecretRefFactory = func(client clientset.Interface, ref *operatorv1alpha1.LocalSecretReference) (clientset.Interface, error) {
		return util.BuildClientFromSecretRef(client, ref)
	}
)

// containAllTasks checks if all tasks in the subset are present in the tasks slice.
// Returns an error if any subset task is not found; nil otherwise.
func containAllTasks(tasks, subset []workflow.Task) error {
	for _, subsetTask := range subset {
		found := false
		for _, task := range tasks {
			found = deepEqualTasks(task, subsetTask) == nil
			if found {
				break
			}
		}
		if !found {
			return fmt.Errorf("subset task %v not found in tasks", subsetTask)
		}
	}
	return nil
}

// deepEqualTasks compares two workflow.Task instances for deep equality.
// Returns an error if they are not equal; nil if they are equal.
func deepEqualTasks(t1, t2 workflow.Task) error {
	if t1.Name != t2.Name {
		return fmt.Errorf("expected t1 name %s, but got %s", t2.Name, t1.Name)
	}

	if t1.RunSubTasks != t2.RunSubTasks {
		return fmt.Errorf("expected t1 RunSubTasks flag %t, but got %t", t2.RunSubTasks, t1.RunSubTasks)
	}

	if len(t1.Tasks) != len(t2.Tasks) {
		return fmt.Errorf("expected t1 tasks length %d, but got %d", len(t2.Tasks), len(t1.Tasks))
	}

	for index := range t1.Tasks {
		err := deepEqualTasks(t1.Tasks[index], t2.Tasks[index])
		if err != nil {
			return fmt.Errorf("unexpected error; tasks are not equal, got %v", err)
		}
	}

	if reflect.ValueOf(t1.Skip).Pointer() != reflect.ValueOf(t2.Skip).Pointer() {
		return fmt.Errorf("expected t1 Skip func %v, but got %v", reflect.ValueOf(t2.Skip).Pointer(), reflect.ValueOf(t1.Skip).Pointer())
	}
	if reflect.ValueOf(t1.Run).Pointer() != reflect.ValueOf(t2.Run).Pointer() {
		return fmt.Errorf("expected t1 Run func %v, but got %v", reflect.ValueOf(t2.Run).Pointer(), reflect.ValueOf(t1.Run).Pointer())
	}

	return nil
}
