package testing

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// NewSingleClusterInformerManagerByRS will build a fake SingleClusterInformerManager and can add resource.
func NewSingleClusterInformerManagerByRS(src string, obj runtime.Object) genericmanager.SingleClusterInformerManager {
	c := fakedynamic.NewSimpleDynamicClient(scheme.Scheme, obj)
	m := genericmanager.NewSingleClusterInformerManager(c, 0, nil)
	m.Lister(corev1.SchemeGroupVersion.WithResource(src))
	m.Start()
	m.WaitForCacheSync()
	return m
}
