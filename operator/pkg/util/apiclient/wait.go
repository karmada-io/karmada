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

// NewKarmadaWaiter reurn a karmada waiter, the rest config is to create crd client or aggregate client.
func NewKarmadaWaiter(config *rest.Config, client clientset.Interface, timeout time.Duration) Waiter {
	return &KarmadaWaiter{
		karmadaConfig: config,
		client:        client,
		timeout:       timeout,
	}
}

// WaitForAPI waits for the API Server's /healthz endpoint to report "ok"
func (w *KarmadaWaiter) WaitForAPI() error {
	return wait.PollImmediate(APICallRetryInterval, w.timeout, func() (bool, error) {
		healthStatus := 0
		w.client.Discovery().RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			return false, nil
		}

		return true, nil
	})
}

// WaitForAPIService waits for the APIService condition to become "true"
func (w *KarmadaWaiter) WaitForAPIService(name string) error {
	aggregateClient, err := aggregator.NewForConfig(w.karmadaConfig)
	if err != nil {
		return err
	}

	err = wait.PollImmediate(APICallRetryInterval, w.timeout, func() (done bool, err error) {
		apiService, err := aggregateClient.ApiregistrationV1().APIServices().Get(context.TODO(), name, metav1.GetOptions{})
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
	return wait.PollImmediate(APICallRetryInterval, w.timeout, func() (bool, error) {
		listOpts := metav1.ListOptions{LabelSelector: label}
		pods, err := w.client.CoreV1().Pods(namespace).List(context.TODO(), listOpts)
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
	return wait.PollImmediate(APICallRetryInterval, w.timeout, func() (bool, error) {
		listOpts := metav1.ListOptions{LabelSelector: label}
		pods, err := w.client.CoreV1().Pods(namespace).List(context.TODO(), listOpts)
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

// TryRunCommand runs a function a maximum of failureThreshold times, and retries on error. If failureThreshold is hit; the last error is returned
func TryRunCommand(f func() error, failureThreshold int) error {
	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   2, // double the timeout for every failure
		Steps:    failureThreshold,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := f()
		if err != nil {
			// Retry until the timeout
			return false, nil
		}
		// The last f() call was a success, return cleanly
		return true, nil
	})
}

func isPodRunning(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return false
	}

	for _, condtion := range pod.Status.Conditions {
		if condtion.Type == corev1.PodReady && condtion.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
