package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateHPA create HPA.
func CreateHPA(client kubernetes.Interface, hpa *autoscalingv2.HorizontalPodAutoscaler) {
	ginkgo.By(fmt.Sprintf("Creating HPA(%s/%s)", hpa.Namespace, hpa.Name), func() {
		_, err := client.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveHPA delete HPA.
func RemoveHPA(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing HPA(%s/%s)", namespace, name), func() {
		err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
