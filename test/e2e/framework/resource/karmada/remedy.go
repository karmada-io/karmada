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
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateRemedy create Remedy with karmada client.
func CreateRemedy(client karmada.Interface, remedy *remedyv1alpha1.Remedy) {
	ginkgo.By(fmt.Sprintf("Creating Remedy(%s)", remedy.Name), func() {
		_, err := client.RemedyV1alpha1().Remedies().Create(context.TODO(), remedy, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveRemedy delete Remedy with karmada client.
func RemoveRemedy(client karmada.Interface, name string) {
	err := client.RemedyV1alpha1().Remedies().Delete(context.TODO(), name, metav1.DeleteOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}
