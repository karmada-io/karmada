package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
		foregroundDelete := metav1.DeletePropagationForeground
		err := client.BatchV1().Jobs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{PropagationPolicy: &foregroundDelete})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitJobPresentOnClusterFitWith wait job present on member clusters sync with fit func.
func WaitJobPresentOnClusterFitWith(cluster, namespace, name string, fit func(job *batchv1.Job) bool) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for job(%s/%s) synced on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		dep, err := clusterClient.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return fit(dep)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitJobPresentOnClustersFitWith wait job present on cluster sync with fit func.
func WaitJobPresentOnClustersFitWith(clusters []string, namespace, name string, fit func(job *batchv1.Job) bool) {
	ginkgo.By(fmt.Sprintf("Waiting for job(%s/%s) synced on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitJobPresentOnClusterFitWith(clusterName, namespace, name, fit)
		}
	})
}

// WaitJobDisappearOnCluster wait job disappear on cluster until timeout.
func WaitJobDisappearOnCluster(cluster, namespace, name string) {
	clusterClient := GetClusterClient(cluster)
	gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for job(%s/%s) disappear on cluster(%s)", namespace, name, cluster)
	gomega.Eventually(func() bool {
		_, err := clusterClient.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return false
		}
		if apierrors.IsNotFound(err) {
			return true
		}

		klog.Errorf("Failed to get job(%s/%s) on cluster(%s), err: %v", namespace, name, cluster, err)
		return false
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}

// WaitJobDisappearOnClusters wait job disappear on member clusters until timeout.
func WaitJobDisappearOnClusters(clusters []string, namespace, name string) {
	ginkgo.By(fmt.Sprintf("Check if job(%s/%s) diappeare on member clusters", namespace, name), func() {
		for _, clusterName := range clusters {
			WaitJobDisappearOnCluster(clusterName, namespace, name)
		}
	})
}

// UpdateJobWithPatchBytes update job with patch bytes.
func UpdateJobWithPatchBytes(client kubernetes.Interface, namespace, name string, patchBytes []byte, patchType types.PatchType) {
	ginkgo.By(fmt.Sprintf("Updating job(%s/%s)", namespace, name), func() {
		_, err := client.BatchV1().Jobs(namespace).Patch(context.TODO(), name, patchType, patchBytes, metav1.PatchOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
