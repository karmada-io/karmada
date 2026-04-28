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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateDaemonSet create DaemonSet.
func CreateDaemonSet(client kubernetes.Interface, daemonSet *appsv1.DaemonSet) {
	ginkgo.By(fmt.Sprintf("Creating DaemonSet(%s/%s)", daemonSet.Namespace, daemonSet.Name), func() {
		_, err := client.AppsV1().DaemonSets(daemonSet.Namespace).Create(context.TODO(), daemonSet, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveDaemonSet delete DaemonSet.
func RemoveDaemonSet(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing DaemonSet(%s/%s)", namespace, name), func() {
		err := client.AppsV1().DaemonSets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
