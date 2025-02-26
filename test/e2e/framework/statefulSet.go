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

// CreateStatefulSet create StatefulSet.
func CreateStatefulSet(client kubernetes.Interface, statefulSet *appsv1.StatefulSet) {
	ginkgo.By(fmt.Sprintf("Creating StatefulSet(%s/%s)", statefulSet.Namespace, statefulSet.Name), func() {
		_, err := client.AppsV1().StatefulSets(statefulSet.Namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveStatefulSet delete StatefulSet.
func RemoveStatefulSet(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing StatefulSet(%s/%s)", namespace, name), func() {
		err := client.AppsV1().StatefulSets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// UpdateStatefulSetReplicas update statefulSet's replicas.
func UpdateStatefulSetReplicas(client kubernetes.Interface, statefulSet *appsv1.StatefulSet, replicas int32) {
	ginkgo.By(fmt.Sprintf("Updating StatefulSet(%s/%s)'s replicas to %d", statefulSet.Namespace, statefulSet.Name, replicas), func() {
		statefulSet.Spec.Replicas = &replicas
		gomega.Eventually(func() error {
			_, err := client.AppsV1().StatefulSets(statefulSet.Namespace).Update(context.TODO(), statefulSet, metav1.UpdateOptions{})
			return err
		}, PollTimeout, PollInterval).ShouldNot(gomega.HaveOccurred())
	})
}
