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

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateResourceInterpreterCustomization creates ResourceInterpreterCustomization with karmada client.
func CreateResourceInterpreterCustomization(client karmada.Interface, customization *configv1alpha1.ResourceInterpreterCustomization) {
	ginkgo.By(fmt.Sprintf("Creating ResourceInterpreterCustomization(%s)", customization.Name), func() {
		_, err := client.ConfigV1alpha1().ResourceInterpreterCustomizations().Create(context.TODO(), customization, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// DeleteResourceInterpreterCustomization deletes ResourceInterpreterCustomization with karmada client.
func DeleteResourceInterpreterCustomization(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Deleting ResourceInterpreterCustomization(%s)", name), func() {
		err := client.ConfigV1alpha1().ResourceInterpreterCustomizations().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
