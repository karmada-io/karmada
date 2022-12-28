package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func podStatus(pod *corev1.Pod) string {
	for _, value := range pod.Status.ContainerStatuses {
		if pod.Status.Phase == corev1.PodRunning {
			if value.State.Waiting != nil {
				return value.State.Waiting.Reason
			}
			if value.State.Waiting == nil {
				return string(corev1.PodRunning)
			}
			return "Error"
		}
		if pod.ObjectMeta.DeletionTimestamp != nil {
			return "Terminating"
		}
	}
	return pod.Status.ContainerStatuses[0].State.Waiting.Reason
}

func isPodReady(c *kubernetes.Clientset, n, p string) wait.ConditionFunc {
	return func() (done bool, err error) {
		pod, err := c.CoreV1().Pods(n).Get(context.TODO(), p, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if pod.Status.Phase == corev1.PodPending && len(pod.Status.ContainerStatuses) == 0 {
			klog.Warningf("Pod: %s not ready. status: %v", pod.Name, corev1.PodPending)
			return false, nil
		}

		for _, v := range pod.Status.Conditions {
			switch v.Type {
			case corev1.PodReady:
				if v.Status == corev1.ConditionTrue {
					klog.Infof("pod: %s is ready. status: %v", pod.Name, podStatus(pod))
					return true, nil
				}
				klog.Warningf("Pod: %s not ready. status: %v", pod.Name, podStatus(pod))
				return false, nil
			default:
				continue
			}
		}
		return false, err
	}
}

// waitPodReady  Poll up to timeout seconds for pod to enter running state.
// Returns an error if the pod never enters the running state.
func waitPodReady(c *kubernetes.Clientset, namespaces, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, isPodReady(c, namespaces, podName))
}

// WaitPodReady wait pod ready
func WaitPodReady(c *kubernetes.Clientset, namespace, selector string, timeout int) error {
	// Wait 3 second
	time.Sleep(3 * time.Second)
	pods, err := c.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods in %s with selector %s", namespace, selector)
	}

	for _, pod := range pods.Items {
		if err = waitPodReady(c, namespace, pod.Name, time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}

	return nil
}

// WaitEtcdReplicasetInDesired Wait Etcd Ready
func WaitEtcdReplicasetInDesired(replicas int32, c *kubernetes.Clientset, namespace, selector string, timeout int) error {
	if err := wait.PollImmediate(time.Second, time.Duration(timeout)*time.Second, func() (done bool, err error) {
		pods, e := c.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector})
		if e != nil {
			return false, nil
		}
		if int32(len(pods.Items)) == replicas {
			klog.Infof("Etcd desired replicaset is %v, currently: %v", replicas, len(pods.Items))
			return true, nil
		}
		klog.Warningf("Etcd desired replicaset is %v, currently: %v", replicas, len(pods.Items))
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}
