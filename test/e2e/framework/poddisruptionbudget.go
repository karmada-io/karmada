package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreatePodDisruptionBudget creates PodDisruptionBudget.
func CreatePodDisruptionBudget(client kubernetes.Interface, pdb *policyv1.PodDisruptionBudget) {
	ginkgo.By(fmt.Sprintf("Creating PodDisruptionBudget(%s/%s)", pdb.Namespace, pdb.Name), func() {
		_, err := client.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemovePodDisruptionBudget deletes PodDisruptionBudget.
func RemovePodDisruptionBudget(client kubernetes.Interface, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing PodDisruptionBudget(%s/%s)", namespace, name), func() {
		err := client.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
