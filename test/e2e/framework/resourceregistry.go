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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateResourceRegistry create ResourceRegistry with karmada client.
func CreateResourceRegistry(client karmada.Interface, rr *searchv1alpha1.ResourceRegistry) {
	ginkgo.By(fmt.Sprintf("Creating ResourceRegistry(%s)", rr.Name), func() {
		_, err := client.SearchV1alpha1().ResourceRegistries().Create(context.TODO(), rr, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveResourceRegistry delete ResourceRegistry with karmada client.
func RemoveResourceRegistry(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing ResourceRegistry(%s)", name), func() {
		err := client.SearchV1alpha1().ResourceRegistries().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateResourceRegistry patch ResourceRegistry with karmada client.
func UpdateResourceRegistry(client karmada.Interface, rr *searchv1alpha1.ResourceRegistry) {
	ginkgo.By(fmt.Sprintf("Update ResourceRegistry(%s)", rr.Name), func() {
		_, err := client.SearchV1alpha1().ResourceRegistries().Update(context.TODO(), rr, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
