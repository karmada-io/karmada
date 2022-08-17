package proxy

import (
	"context"
	"net/http"

	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

type cacheProxy struct {
	store *store.MultiClusterCache
}

func (p *cacheProxy) connect(ctx context.Context) http.Handler {
	return nil
}

func newCacheProxy(store *store.MultiClusterCache) *cacheProxy {
	return &cacheProxy{
		store: store,
	}
}
