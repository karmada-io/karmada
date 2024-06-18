/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	if err := wait.PollUntilContextTimeout(context.TODO(), time.Second, timeout, true, func(ctx context.Context) (done bool, err error) {
		apiService, e := c.ApiregistrationV1().APIServices().Get(ctx, name, metav1.GetOptions{})
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
