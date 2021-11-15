package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateJob create Job.
func CreateJob(client kubernetes.Interface, job *batchv1.Job) {
	ginkgo.By(fmt.Sprintf("Creating Job(%s/%s)", job.Namespace, job.Name), func() {
		_, err := client.BatchV1().Jobs(job.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// GetJob get Job.
func GetJob(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Get job(%s)", name), func() {
		_, err := client.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveJob delete Job.
func RemoveJob(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Job(%s/%s)", namespace, name), func() {
		err := client.BatchV1().Jobs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
