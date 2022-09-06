package benchmark

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/test/helper"
)

func TestBenchmark(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Benchmark Suite")
}

const (
	// RandomStrLength represents the random string length to combine names.
	RandomStrLength = 5
)

var (
	restConfig    *rest.Config
	kubeClient    kubernetes.Interface
	karmadaClient karmada.Interface

	// options
	karmadaContext         = ""
	testNamespace          = metav1.NamespaceDefault
	qps            float64 = 1000
	burst                  = 2000
	samplingConfig         = gmeasure.SamplingConfig{
		N:           0,
		Duration:    time.Minute * 5,
		NumParallel: 10,
	}
)

func init() {
	flag.StringVar(&karmadaContext, "context", karmadaContext, "Name of the cluster context in control plane kubeconfig file.")
	flag.StringVar(&testNamespace, "test-namespace", testNamespace, fmt.Sprintf("test namespace. (default %v)", testNamespace))
	flag.IntVar(&samplingConfig.N, "sampling-max-number", samplingConfig.N, fmt.Sprintf("the maximum number of samples to record. (default %v)", samplingConfig.N))
	flag.DurationVar(&samplingConfig.Duration, "sampling-max-time", samplingConfig.Duration, fmt.Sprintf("The maximum amount of time to spend recording samples. (default %v)", samplingConfig.Duration))
	flag.IntVar(&samplingConfig.NumParallel, "sampling-number-parallel", samplingConfig.NumParallel, fmt.Sprintf("The number of parallel workers to spin up to record samples. (default %v)", samplingConfig.NumParallel))
	flag.Float64Var(&qps, "kube-api-qps", qps, fmt.Sprintf("QPS to use while talking with kubernetes apiserver. (default %v)", qps))
	flag.IntVar(&burst, "kube-api-burst", burst, fmt.Sprintf("Burst to use while talking with kubernetes apiserver. (default %v)", burst))
}

var _ = ginkgo.BeforeSuite(func() {
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")
	gomega.Expect(kubeconfig).ShouldNot(gomega.BeEmpty())

	restConfig, err = helper.LoadRESTClientConfig(kubeconfig, karmadaContext)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	restConfig.QPS = float32(qps)
	restConfig.Burst = burst

	kubeClient = kubernetes.NewForConfigOrDie(restConfig)
	karmadaClient = karmada.NewForConfigOrDie(restConfig)
})

var _ = ginkgo.AfterSuite(func() {
	// suite tear down, such as cleanup karmada environment.
})
