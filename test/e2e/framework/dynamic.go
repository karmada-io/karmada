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

package framework

import (
	"context"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

// GetResourceNames list resources and return their names.
func GetResourceNames(client dynamic.ResourceInterface) sets.Set[string] {
	list, err := client.List(context.TODO(), metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	names := sets.New[string]()
	for _, item := range list.Items {
		names.Insert(item.GetName())
	}
	return names
}

// GetAnyResourceOrFail list resources and return anyone. Failed if listing empty.
func GetAnyResourceOrFail(client dynamic.ResourceInterface) *unstructured.Unstructured {
	list, err := client.List(context.TODO(), metav1.ListOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(list.Items).ShouldNot(gomega.BeEmpty())

	if len(list.Items) == 1 {
		return &list.Items[0]
	}
	return &list.Items[rand.Intn(len(list.Items))]
}
