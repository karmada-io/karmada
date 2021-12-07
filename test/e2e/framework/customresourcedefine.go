package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

// CreateCRD create CustomResourceDefinition with dynamic client.
func CreateCRD(client dynamic.Interface, crd *apiextensionsv1.CustomResourceDefinition) {
	ginkgo.By(fmt.Sprintf("Creating crd(%s)", crd.Name), func() {
		unstructObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		_, err = client.Resource(crdGVR).Create(context.TODO(), &unstructured.Unstructured{Object: unstructObj}, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// GetCRD get CustomResourceDefinition with dynamic client.
func GetCRD(client dynamic.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Get crd(%s)", name), func() {
		_, err := client.Resource(crdGVR).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveCRD delete CustomResourceDefinition with dynamic client.
func RemoveCRD(client dynamic.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing crd(%s)", name), func() {
		err := client.Resource(crdGVR).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitCRDPresentOnClusters wait CustomResourceDefinition present on clusters until timeout.
func WaitCRDPresentOnClusters(client karmada.Interface, clusters []string, crdAPIVersion, crdKind string) {
	// Check CRD enablement from cluster objects instead of member clusters.
	// After CRD installed on member cluster, the cluster status controller takes at most cluster-status-update-frequency
	// time to collect the API list, before that the scheduler will filter out the cluster from scheduling.
	ginkgo.By(fmt.Sprintf("Check if crd(%s/%s) present on member clusters", crdAPIVersion, crdKind), func() {
		for _, clusterName := range clusters {
			klog.Infof("Waiting for crd present on cluster(%s)", clusterName)
			gomega.Eventually(func(g gomega.Gomega) (bool, error) {
				cluster, err := fetchCluster(client, clusterName)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				return helper.IsAPIEnabled(cluster.Status.APIEnablements, crdAPIVersion, crdKind), nil
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		}
	})
}

// WaitCRDDisappearedOnClusters wait CustomResourceDefinition disappear on clusters until timeout.
func WaitCRDDisappearedOnClusters(clusters []string, crdName string) {
	ginkgo.By("Check if crd disappeared on member clusters", func() {
		for _, clusterName := range clusters {
			clusterDynamicClient := GetClusterDynamicClient(clusterName)
			gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

			klog.Infof("Waiting for crd(%s) disappeared on cluster(%s)", crdName, clusterName)
			gomega.Eventually(func() bool {
				_, err := clusterDynamicClient.Resource(crdGVR).Get(context.TODO(), crdName, metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		}
	})
}
