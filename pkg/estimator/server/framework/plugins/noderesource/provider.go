/*
Copyright 2026 The Karmada Authors.

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

package noderesource

import (
	"context"
	"fmt"
	"sync"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
)

// CapacityProvider supplies additional replica capacity beyond existing nodes.
// Providers are composed inside NodeResourceEstimator; their results are summed
// on top of the built-in existing-node calculation.
type CapacityProvider interface {
	Name() string
	Estimate(ctx context.Context, snapshot *schedcache.Snapshot,
		req *pb.ReplicaRequirements) (int32, *framework.Result)
}

// ProviderFactory creates a CapacityProvider from a framework Handle.
type ProviderFactory func(fh framework.Handle) (CapacityProvider, error)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]ProviderFactory)
)

// RegisterProvider registers a provider factory by name. Intended to be called
// from provider package init(). Names must be unique.
func RegisterProvider(name string, f ProviderFactory) {
	providersMu.Lock()
	defer providersMu.Unlock()
	if _, exists := providers[name]; exists {
		panic(fmt.Sprintf("capacity provider %q already registered", name))
	}
	providers[name] = f
}

// LookupProvider returns the factory for the given name and whether it exists.
func LookupProvider(name string) (ProviderFactory, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	f, ok := providers[name]
	return f, ok
}

// RegisteredProviders returns all registered provider names.
func RegisteredProviders() []string {
	providersMu.RLock()
	defer providersMu.RUnlock()
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	return names
}
