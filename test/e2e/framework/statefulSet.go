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
		}, pollTimeout, pollInterval).ShouldNot(gomega.HaveOccurred())
	})
}
