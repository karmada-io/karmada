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
