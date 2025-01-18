/*
Copyright 2022 The Karmada Authors.

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
)

// variables for test
var (
	PodGVK  = corev1.SchemeGroupVersion.WithKind("Pod")
	NodeGVK = corev1.SchemeGroupVersion.WithKind("Node")

	PodGVR     = corev1.SchemeGroupVersion.WithResource("pods")
	NodeGVR    = corev1.SchemeGroupVersion.WithResource("nodes")
	SecretGVR  = corev1.SchemeGroupVersion.WithResource("secret")
	ClusterGVR = clusterv1alpha1.SchemeGroupVersion.WithResource("cluster")

	PodSelector = searchv1alpha1.ResourceSelector{APIVersion: PodGVK.GroupVersion().String(), Kind: PodGVK.Kind}

	PodSelectorWithNS1 = searchv1alpha1.ResourceSelector{APIVersion: PodGVK.GroupVersion().String(), Kind: PodGVK.Kind, Namespace: "ns1"}

	PodSelectorWithNS2 = searchv1alpha1.ResourceSelector{APIVersion: PodGVK.GroupVersion().String(), Kind: PodGVK.Kind, Namespace: "ns2"}

	NodeSelector = searchv1alpha1.ResourceSelector{APIVersion: NodeGVK.GroupVersion().String(), Kind: NodeGVK.Kind}

	RestMapper *meta.DefaultRESTMapper
)

func init() {
	RestMapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	RestMapper.Add(PodGVK, meta.RESTScopeNamespace)
	RestMapper.Add(NodeGVK, meta.RESTScopeRoot)
}
