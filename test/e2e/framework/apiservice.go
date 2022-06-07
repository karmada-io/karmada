package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	v1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// CreateAPIService create a apiservice
func CreateAPIService(client aggregatorclient.Interface, aPIService *v1.APIService) {
	ginkgo.By(fmt.Sprintf("Creating APIService(%s)", aPIService.Name), func() {
		_, err := client.ApiregistrationV1().APIServices().Create(context.TODO(), aPIService, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// RemoveAPIService delete a apiservice in cluster
func RemoveAPIService(client aggregatorclient.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Removing APIService(%s)", name), func() {
		err := client.ApiregistrationV1().APIServices().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// WaitAPIServiceDisappearOnCluster wait apiservice disappear on cluster until timeout.
func WaitAPIServiceDisappearOnCluster(cluster, name string) {
	aggrclient := GetClusterAggregatorClient(cluster)
	gomega.Expect(aggrclient).ShouldNot(gomega.BeNil())

	klog.Infof("Waiting for apiservice disappear on cluster(%s)", cluster)
	gomega.Eventually(func() bool {
		_, err := aggrclient.ApiregistrationV1().APIServices().Get(context.TODO(), name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, pollTimeout, pollInterval).Should(gomega.Equal(true))
}
