package karmada

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1helper "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1/helper"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// WaitAPIServiceReady wait the api service condition true
func WaitAPIServiceReady(c *aggregator.Clientset, name string, timeout time.Duration) error {
	if err := wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		apiService, e := c.ApiregistrationV1().APIServices().Get(context.TODO(), name, metav1.GetOptions{})
		if e != nil {
			return false, nil
		}
		if apiregistrationv1helper.IsAPIServiceConditionTrue(apiService, apiregistrationv1.Available) {
			return true, nil
		}

		klog.Infof("Waiting for APIService(%s) condition(%s), will try", name, apiregistrationv1.Available)
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}
