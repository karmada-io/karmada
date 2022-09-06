package benchmark

import (
	"context"
	"flag"
	"fmt"
	gorand "math/rand"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	"go.uber.org/atomic"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/lifted"
	"github.com/karmada-io/karmada/test/e2e/framework"
)

var (
	// options
	createResourceRegistry = true
)

func init() {
	flag.BoolVar(&createResourceRegistry, "create-resource-registry", createResourceRegistry, fmt.Sprintf("Create createResourceRegistry before test or not. (default %v)", burst))
}

var _ = ginkgo.Describe("Search bench test", ginkgo.Ordered, func() {
	var (
		pickPod        func(int) types.NamespacedName
		isMultiCluster bool
	)

	ginkgo.BeforeAll(func() {
		var podPool []types.NamespacedName
		podPool, isMultiCluster = initPodPool(50000)
		pickPod = func(i int) types.NamespacedName {
			return podPool[i%len(podPool)]
		}

		if isMultiCluster && createResourceRegistry {
			rrList, err := karmadaClient.SearchV1alpha1().ResourceRegistries().List(context.TODO(), metav1.ListOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(rrList.Items).Should(gomega.BeEmpty(), "ResourceRegistry exists, clean it before test")

			rr := &searchv1alpha1.ResourceRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "benchmark-" + rand.String(RandomStrLength),
				},
				Spec: searchv1alpha1.ResourceRegistrySpec{
					ResourceSelectors: []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			}
			framework.CreateResourceRegistry(karmadaClient, rr)

			ginkgo.DeferCleanup(func() {
				framework.RemoveResourceRegistry(karmadaClient, rr.Name)
			})
		}

		if isMultiCluster {
			// wait proxy is ready
			gomega.Eventually(func(g gomega.Gomega) {
				list, err := kubeClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{Limit: 500})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())
				g.Expect(list.Items).ShouldNot(gomega.BeEmpty())
			}, time.Minute*10, time.Second).Should(gomega.Succeed())
		}
	})

	ginkgo.It("list pods", ginkgo.Label("measurement"), func() {
		klog.Info("Start test: list pods")
		measureDuration("List pods", func(int, *gmeasure.Stopwatch) error {
			// If we list with resourceVersion=0, it will return all the pods from cache (limit doesn't work).
			_, err := kubeClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{Limit: 500})
			return err
		})
	})

	ginkgo.It("get pod", ginkgo.Label("measurement"), func() {
		klog.Info("Start test: get pod")
		measureDuration("Get pod", func(idx int, _ *gmeasure.Stopwatch) error {
			pod := pickPod(idx)
			_, err := kubeClient.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{ResourceVersion: "0"})
			return err
		})
	})

	ginkgo.It("update pod", ginkgo.Label("measurement"), func() {
		klog.Info("Start test: update pod")
		measureDuration("Update pod", func(idx int, sw *gmeasure.Stopwatch) error {
			pod := pickPod(idx)
			return wait.PollImmediate(time.Millisecond*10, time.Second*10, func() (done bool, err error) {
				pod, err := kubeClient.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				sw.Reset()

				// We don't modify pod here, reducing conflict error. The call chain is same as update a modified one,
				// except writing to etcd. Reference: vendor/k8s.io/apiserver/pkg/storage/etcd3/store.go:404
				// Etcd performance is not in our test scope.
				_, err = kubeClient.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
				if errors.IsConflict(err) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				return true, nil
			})
		})
	})
})

func measureDuration(name string, f func(int, *gmeasure.Stopwatch) error) {
	total := atomic.Int32{}
	errCount := atomic.Int32{}

	experiment := gmeasure.NewExperiment(name)
	ginkgo.AddReportEntry(name, experiment)

	experiment.Sample(func(idx int) {
		total.Inc()
		sw := experiment.NewStopwatch()
		err := f(idx, sw)
		if err != nil {
			klog.Error(err.Error())
			errCount.Inc()
		} else {
			sw.Record(name)
		}
	}, samplingConfig)

	klog.Infof(name+" [errors]: %v/%v", errCount.Load(), total.Load())
}

// initPodPool pick up about n pods into pool. Actually will not pick exactly n number of pods, usually less than n.
func initPodPool(n int) ([]types.NamespacedName, bool) {
	pods := make([]types.NamespacedName, 0, n)
	appendLock := sync.Mutex{}
	appendPodsFunc := func(podList *corev1.PodList) {
		appendLock.Lock()
		defer appendLock.Unlock()
		for _, pod := range podList.Items {
			pods = append(pods, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})
		}
	}

	clusterList, err := karmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if errors.IsNotFound(err) {
		// This is a single cluster.
		klog.Info("Pick pods from single cluster")
		podList, err := kubeClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{Limit: int64(n)})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		appendPodsFunc(podList)
		klog.Infof("picks up %v pods", len(pods))
		return pods, false
	}
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	// get from multi clusters
	clusters := make([]string, 0, len(clusterList.Items))
	for i := range clusterList.Items {
		cluster := &clusterList.Items[i]
		if cluster.Spec.SyncMode == clusterv1alpha1.Push {
			clusters = append(clusters, cluster.Name)
		}
	}

	klog.Infof("Pick pods from %v clusters", len(clusters))
	controlPlaneClient := gclient.NewForConfigOrDie(restConfig)
	limit := int64(n / len(clusters))
	lifted.NewParallelizer(16).Until(context.TODO(), len(clusters), func(i int) {
		cluster := clusters[i]
		clusterClient, err := util.NewClusterClientSet(cluster, controlPlaneClient, nil)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		podList, err := clusterClient.KubeClient.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{Limit: limit})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		appendPodsFunc(podList)
	})

	gorand.Shuffle(len(pods), func(i, j int) {
		pods[i], pods[j] = pods[j], pods[i]
	})
	klog.Infof("picks up %v pods", len(pods))
	return pods, true
}
