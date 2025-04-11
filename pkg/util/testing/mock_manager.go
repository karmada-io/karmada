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

package testing

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// NewSingleClusterInformerManagerByRS will build a fake SingleClusterInformerManager and can add resource.
func NewSingleClusterInformerManagerByRS(src string, obj runtime.Object) genericmanager.SingleClusterInformerManager {
	c := fakedynamic.NewSimpleDynamicClient(scheme.Scheme, obj)
	m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
	m.Lister(corev1.SchemeGroupVersion.WithResource(src))
	m.Start()
	m.WaitForCacheSync()
	return m
}
