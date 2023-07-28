package framework

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	pkgutil "github.com/karmada-io/karmada/pkg/util"
)

// CreateResourceInterpreterCustomization creates ResourceInterpreterCustomization with karmada client.
func CreateResourceInterpreterCustomization(client karmada.Interface, kubeClient kubeclient.Interface,
	customization *configv1alpha1.ResourceInterpreterCustomization) {
	ginkgo.By(fmt.Sprintf("Creating ResourceInterpreterCustomization(%s)", customization.Name), func() {
		var err error
		func() {
			defer pkgutil.PrintTimeCost("CreateResourceInterpreterCustomizationE2ETest")()
			_, err = client.ConfigV1alpha1().ResourceInterpreterCustomizations().Create(context.TODO(), customization, metav1.CreateOptions{})
		}()

		svc, errSvc := kubeClient.CoreV1().Services("karmada-system").Get(context.TODO(), "karmada-webhook", metav1.GetOptions{})
		ep, errEp := kubeClient.CoreV1().Endpoints("karmada-system").Get(context.TODO(), "karmada-webhook", metav1.GetOptions{})
		klog.Infof("karmada-webhook svc: %+v, errSvc: %+v\n\nep: %+v, errEp: %+v", svc, errSvc, ep, errEp)

		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// DeleteResourceInterpreterCustomization deletes ResourceInterpreterCustomization with karmada client.
func DeleteResourceInterpreterCustomization(client karmada.Interface, name string) {
	ginkgo.By(fmt.Sprintf("Deleting ResourceInterpreterCustomization(%s)", name), func() {
		err := client.ConfigV1alpha1().ResourceInterpreterCustomizations().Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}
