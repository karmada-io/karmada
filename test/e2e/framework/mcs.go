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

package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// CreateMultiClusterService creates MultiClusterService with karmada client.
func CreateMultiClusterService(client karmada.Interface, mcs *networkingv1alpha1.MultiClusterService) {
	ginkgo.By(fmt.Sprintf("Creating MultiClusterService(%s/%s)", mcs.Namespace, mcs.Name), func() {
		_, err := client.NetworkingV1alpha1().MultiClusterServices(mcs.Namespace).Create(context.TODO(), mcs, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateMultiClusterService updates MultiClusterService with karmada client.
func UpdateMultiClusterService(client karmada.Interface, mcs *networkingv1alpha1.MultiClusterService) {
	ginkgo.By(fmt.Sprintf("Updating MultiClusterService(%s/%s)", mcs.Namespace, mcs.Name), func() {
		gomega.Eventually(func() error {
			mcsExist, err := client.NetworkingV1alpha1().MultiClusterServices(mcs.Namespace).Get(context.TODO(), mcs.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			mcsExist.Spec = mcs.Spec
			_, err = client.NetworkingV1alpha1().MultiClusterServices(mcsExist.Namespace).Update(context.TODO(), mcsExist, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveMultiClusterService deletes MultiClusterService with karmada client.
func RemoveMultiClusterService(client karmada.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing MultiClusterService(%s/%s)", namespace, name), func() {
		err := client.NetworkingV1alpha1().MultiClusterServices(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitMultiClusterServicePresentOnClustersFitWith waits MultiClusterService present on cluster sync with fit func.
func WaitMultiClusterServicePresentOnClustersFitWith(client karmada.Interface, namespace, name string, fit func(mcs *networkingv1alpha1.MultiClusterService) bool) {
	klog.Infof("Waiting for MultiClusterService(%s/%s) fit", namespace, name)
	gomega.Eventually(func() bool {
		mcs, err := client.NetworkingV1alpha1().MultiClusterServices(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		return fit(mcs)
	}, PollTimeout, PollInterval).Should(gomega.Equal(true))
}
