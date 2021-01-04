package scheduler

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

type Scheduler struct {
	DynamicClient dynamic.Interface
	KarmadaClient karmadaclientset.Interface
	KubeClient    kubernetes.Interface
}

func NewScheduler(dynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, kubeClient kubernetes.Interface) *Scheduler {
	return &Scheduler{
		DynamicClient: dynamicClient,
		KarmadaClient: karmadaClient,
		KubeClient:    kubeClient,
	}
}

func (s *Scheduler) Run(stop <-chan struct{}) {
	apiResources, _ := s.KubeClient.Discovery().ServerPreferredResources()
	for _, apiresource := range apiResources {
		klog.Infof("%s %v\n", apiresource.GroupVersion, apiresource.APIResources)
	}
	<-stop
}
