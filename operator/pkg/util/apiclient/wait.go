/*
Copyright 2023 The Karmada Authors.

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

package apiclient

import (
	"context"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1helper "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1/helper"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

const (
	// APICallRetryInterval defines how long kubeadm should wait before retrying a failed API operation
	APICallRetryInterval = 500 * time.Millisecond
)

var (
	// initialBackoffDuration defines the initial duration for the backoff mechanism,
	// set to 5 seconds. This value is used to determine the wait time before retrying
	// a failed command.
	initialBackoffDuration = 5 * time.Second

	// backoffTimeoutFactor is the factor by which the backoff duration is multiplied
	// after each failure. In this case, it is set to 2, meaning the wait time will
	// double with each consecutive failure.
	backoffTimeoutFactor float64 = 2
)

// Waiter is an interface for waiting for criteria in Karmada to happen
type Waiter interface {
	// WaitForAPI waits for the API Server's /healthz endpoint to become "ok"
	WaitForAPI() error
	// WaitForAPIService waits for the APIService condition to become "true"
	WaitForAPIService(name string) error
	// WaitForPods waits for Pods in the namespace to become Ready
	WaitForPods(label, namespace string) error
	// WaitForSomePods waits for the specified number of Pods in the namespace to become Ready
	WaitForSomePods(label, namespace string, podNum int32) error
	// SetTimeout adjusts the timeout to the specified duration
	SetTimeout(timeout time.Duration)
}

// KarmadaWaiter is an implementation of Waiter that is backed by a Kubernetes client
type KarmadaWaiter struct {
	karmadaConfig *rest.Config
	client        clientset.Interface
	timeout       time.Duration
}

// NewKarmadaWaiter returns a karmada waiter, the rest config is to create crd client or aggregate client.
func NewKarmadaWaiter(config *rest.Config, client clientset.Interface, timeout time.Duration) Waiter {
	return &KarmadaWaiter{
		karmadaConfig: config,
		client:        client,
		timeout:       timeout,
	}
}

// WaitForAPI waits for the API Server's /healthz endpoint to report "ok"
func (w *KarmadaWaiter) WaitForAPI() error {
	return wait.PollUntilContextTimeout(context.TODO(), APICallRetryInterval, w.timeout, true, func(ctx context.Context) (bool, error) {
		healthStatus := 0
		w.client.Discovery().RESTClient().Get().AbsPath("/healthz").Do(ctx).StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			return false, nil
		}

		return true, nil
	})
}

var aggregateClientFromConfigBuilder = func(karmadaConfig *rest.Config) (aggregator.Interface, error) {
	return aggregator.NewForConfig(karmadaConfig)
}

// WaitForAPIService waits for the APIService condition to become "true"
func (w *KarmadaWaiter) WaitForAPIService(name string) error {
	aggregateClient, err := aggregateClientFromConfigBuilder(w.karmadaConfig)
	if err != nil {
		return err
	}

	err = wait.PollUntilContextTimeout(context.TODO(), APICallRetryInterval, w.timeout, true, func(ctx context.Context) (done bool, err error) {
		apiService, err := aggregateClient.ApiregistrationV1().APIServices().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if apiregistrationv1helper.IsAPIServiceConditionTrue(apiService, apiregistrationv1.Available) {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// WaitForPods will lookup pods with the given label and wait until they are all
// reporting status as running.
func (w *KarmadaWaiter) WaitForPods(label, namespace string) error {
	lastKnownPodNumber := -1
	return wait.PollUntilContextTimeout(context.TODO(), APICallRetryInterval, w.timeout, true, func(ctx context.Context) (bool, error) {
		listOpts := metav1.ListOptions{LabelSelector: label}
		pods, err := w.client.CoreV1().Pods(namespace).List(ctx, listOpts)
		if err != nil {
			return false, nil
		}

		if lastKnownPodNumber != len(pods.Items) {
			lastKnownPodNumber = len(pods.Items)
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		for _, pod := range pods.Items {
			if !isPodRunning(pod) {
				return false, nil
			}
		}

		return true, nil
	})
}

// WaitForSomePods lookup pods with the given label and wait until desired number of pods
// reporting status as running.
func (w *KarmadaWaiter) WaitForSomePods(label, namespace string, podNum int32) error {
	return wait.PollUntilContextTimeout(context.TODO(), APICallRetryInterval, w.timeout, true, func(ctx context.Context) (bool, error) {
		listOpts := metav1.ListOptions{LabelSelector: label}
		pods, err := w.client.CoreV1().Pods(namespace).List(ctx, listOpts)
		if err != nil {
			return false, nil
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		var expected int32
		for _, pod := range pods.Items {
			if isPodRunning(pod) {
				expected++
			}
		}
		return expected >= podNum, nil
	})
}

// SetTimeout adjusts the timeout to the specified duration
func (w *KarmadaWaiter) SetTimeout(timeout time.Duration) {
	w.timeout = timeout
}

// TryRunCommand runs a function a maximum of failureThreshold times, and
// retries on error. If failureThreshold is hit; the last error is returned.
func TryRunCommand(f func() error, failureThreshold int) error {
	backoff := wait.Backoff{
		Duration: initialBackoffDuration,
		Factor:   backoffTimeoutFactor,
		Steps:    failureThreshold,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := f()
		if err != nil {
			// Retry until the timeout.
			return false, nil
		}
		// The last f() call was a success, return cleanly.
		return true, nil
	})
}

func isPodRunning(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
