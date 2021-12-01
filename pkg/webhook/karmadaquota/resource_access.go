package karmadaquota

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage/etcd3"
	"k8s.io/utils/lru"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/quota/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	v1alpha1lster "github.com/karmada-io/karmada/pkg/generated/listers/quota/v1alpha1"
)

// QuotaAccessor abstracts the get/set logic from the rest of the Evaluator.  This could be a test stub, a straight passthrough,
// or most commonly a series of deconflicting caches.
type QuotaAccessor interface {
	// UpdateQuotaStatus is called to persist final status.  This method should write to persistent storage.
	// An error indicates that write didn't complete successfully.
	UpdateQuotaStatus(newQuota *v1alpha1.KarmadaQuota) error

	// GetQuotas gets all possible quotas for a given namespace
	GetQuotas(namespace string) ([]v1alpha1.KarmadaQuota, error)
}

type quotaAccessor struct {
	client karmadaclientset.Interface

	// lister can list/get quota objects from a shared informer's cache
	lister v1alpha1lster.KarmadaQuotaLister

	// liveLookups holds the last few live lookups we've done to help ammortize cost on repeated lookup failures.
	// This lets us handle the case of latent caches, by looking up actual results for a namespace on cache miss/no results.
	// We track the lookup result here so that for repeated requests, we don't look it up very often.
	liveLookupCache *lru.Cache
	liveTTL         time.Duration
	// updatedQuotas holds a cache of quotas that we've updated.  This is used to pull the "really latest" during back to
	// back quota evaluations that touch the same quota doc.  This only works because we can compare etcd resourceVersions
	// for the same resource as integers.  Before this change: 22 updates with 12 conflicts.  after this change: 15 updates with 0 conflicts
	updatedQuotas *lru.Cache
}

// newQuotaAccessor creates an object that conforms to the QuotaAccessor interface to be used to retrieve quota objects.
func newQuotaAccessor() (*quotaAccessor, error) {
	liveLookupCache := lru.New(100)
	updatedCache := lru.New(100)

	// client and lister will be set when SetInternalKubeClientSet and SetInternalKubeInformerFactory are invoked
	return &quotaAccessor{
		liveLookupCache: liveLookupCache,
		liveTTL:         time.Duration(30 * time.Second),
		updatedQuotas:   updatedCache,
	}, nil
}

func (e *quotaAccessor) UpdateQuotaStatus(newQuota *v1alpha1.KarmadaQuota) error {
	updatedQuota, err := e.client.QuotaV1alpha1().KarmadaQuotas(newQuota.Namespace).UpdateStatus(context.TODO(), newQuota, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	key := newQuota.Namespace + "/" + newQuota.Name
	e.updatedQuotas.Add(key, updatedQuota)
	return nil
}

var etcdVersioner = etcd3.APIObjectVersioner{}

// checkCache compares the passed quota against the value in the look-aside cache and returns the newer
// if the cache is out of date, it deletes the stale entry.  This only works because of etcd resourceVersions
// being monotonically increasing integers
func (e *quotaAccessor) checkCache(quota *v1alpha1.KarmadaQuota) *v1alpha1.KarmadaQuota {
	key := quota.Namespace + "/" + quota.Name
	uncastCachedQuota, ok := e.updatedQuotas.Get(key)
	if !ok {
		return quota
	}
	cachedQuota := uncastCachedQuota.(*v1alpha1.KarmadaQuota)

	if etcdVersioner.CompareResourceVersion(quota, cachedQuota) >= 0 {
		e.updatedQuotas.Remove(key)
		return quota
	}
	return cachedQuota
}

func (e *quotaAccessor) GetQuotas(namespace string) ([]v1alpha1.KarmadaQuota, error) {
	// determine if there are any quotas in this namespace
	// if there are no quotas, we don't need to do anything
	items, err := e.lister.KarmadaQuotas(namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error resolving quota: %v", err)
	}

	// if there are no items held in our indexer, check our live-lookup LRU, if that misses, do the live lookup to prime it.
	if len(items) == 0 {
		lruItemObj, ok := e.liveLookupCache.Get(namespace)
		if !ok || lruItemObj.(liveLookupEntry).expiry.Before(time.Now()) {
			// TODO: If there are multiple operations at the same time and cache has just expired,
			// this may cause multiple List operations being issued at the same time.
			// If there is already in-flight List() for a given namespace, we should wait until
			// it is finished and cache is updated instead of doing the same, also to avoid
			// throttling - see #22422 for details.
			liveList, err := e.client.QuotaV1alpha1().KarmadaQuotas(namespace).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			newEntry := liveLookupEntry{expiry: time.Now().Add(e.liveTTL)}
			for i := range liveList.Items {
				newEntry.items = append(newEntry.items, &liveList.Items[i])
			}
			e.liveLookupCache.Add(namespace, newEntry)
			lruItemObj = newEntry
		}
		lruEntry := lruItemObj.(liveLookupEntry)
		for i := range lruEntry.items {
			items = append(items, lruEntry.items[i])
		}
	}

	resourceQuotas := []v1alpha1.KarmadaQuota{}
	for i := range items {
		quota := items[i]
		quota = e.checkCache(quota)
		// always make a copy.  We're going to muck around with this and we should never mutate the originals
		resourceQuotas = append(resourceQuotas, *quota)
	}

	return resourceQuotas, nil
}
