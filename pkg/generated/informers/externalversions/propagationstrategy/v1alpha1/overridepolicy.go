// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	propagationstrategyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	versioned "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/karmada-io/karmada/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/karmada-io/karmada/pkg/generated/listers/propagationstrategy/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// OverridePolicyInformer provides access to a shared informer and lister for
// OverridePolicies.
type OverridePolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.OverridePolicyLister
}

type overridePolicyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewOverridePolicyInformer constructs a new informer for OverridePolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewOverridePolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredOverridePolicyInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredOverridePolicyInformer constructs a new informer for OverridePolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredOverridePolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PropagationstrategyV1alpha1().OverridePolicies(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PropagationstrategyV1alpha1().OverridePolicies(namespace).Watch(context.TODO(), options)
			},
		},
		&propagationstrategyv1alpha1.OverridePolicy{},
		resyncPeriod,
		indexers,
	)
}

func (f *overridePolicyInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredOverridePolicyInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *overridePolicyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&propagationstrategyv1alpha1.OverridePolicy{}, f.defaultInformer)
}

func (f *overridePolicyInformer) Lister() v1alpha1.OverridePolicyLister {
	return v1alpha1.NewOverridePolicyLister(f.Informer().GetIndexer())
}
