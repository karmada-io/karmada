package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateIngress create Ingress.
func CreateIngress(client kubernetes.Interface, ingress *networkingv1.Ingress) {
	ginkgo.By(fmt.Sprintf("Creating Ingress(%s/%s)", ingress.Namespace, ingress.Name), func() {
		_, err := client.NetworkingV1().Ingresses(ingress.Namespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveIngress delete Ingress.
func RemoveIngress(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Ingress(%s/%s)", namespace, name), func() {
		err := client.NetworkingV1().Ingresses(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
