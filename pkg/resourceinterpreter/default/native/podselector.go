/*
Copyright 2026 The Karmada Authors.

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

package native

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// podSelectorInterpreter is the function that used to parse the label selector of the Pods derived from object.
type podSelectorInterpreter func(object *unstructured.Unstructured) (*metav1.LabelSelector, error)

func getAllDefaultPodSelectorInterpreter() map[schema.GroupVersionKind]podSelectorInterpreter {
	s := make(map[schema.GroupVersionKind]podSelectorInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployPodSelector
	return s
}

func deployPodSelector(object *unstructured.Unstructured) (*metav1.LabelSelector, error) {
	deploy := &appsv1.Deployment{}
	if err := helper.ConvertToTypedObject(object, deploy); err != nil {
		klog.Errorf("Failed to convert object(%s), err %v", object.GroupVersionKind().String(), err)
		return nil, err
	}

	return deploy.Spec.Selector.DeepCopy(), nil
}
