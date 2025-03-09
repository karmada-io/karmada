/*
Copyright 2025 The Karmada Authors.

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

package util

import (
	"sync"

	"k8s.io/client-go/util/flowcontrol"
)

var defaultRateLimiterGetter = &ClusterRateLimiterGetter{}

// GetClusterRateLimiterGetter returns a ClusterRateLimiterGetter.
func GetClusterRateLimiterGetter() *ClusterRateLimiterGetter {
	return defaultRateLimiterGetter
}

// ClusterRateLimiterGetter is responsible for retrieving rate limiters for member clusters.
// It dynamically creates and manages a singleton rate limiter for each member cluster,
// using the preset default QPS and burst values to ensure consistent rate limiting across all clusters.
type ClusterRateLimiterGetter struct {
	limiters map[string]flowcontrol.RateLimiter
	mu       sync.Mutex
	qps      float32
	burst    int
}

// SetDefaultLimits sets the default qps and burst for new rate limiters.
// Do not call this method after calling GetRateLimiter, otherwise the call will not take effect.
// If qps and limit are not set when using Getter,
// it will use 40 and 60 by default (the default qps and burst of '--cluster-api-qps' and '--cluster-api-burst' in karmada-controller-manager).
func (r *ClusterRateLimiterGetter) SetDefaultLimits(qps float32, burst int) *ClusterRateLimiterGetter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.limiters) == 0 {
		r.qps = qps
		r.burst = burst
	}
	return r
}

// GetRateLimiter gets rate limiter by key.
func (r *ClusterRateLimiterGetter) GetRateLimiter(key string) flowcontrol.RateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limiters == nil {
		r.limiters = make(map[string]flowcontrol.RateLimiter)
	}
	limiter, ok := r.limiters[key]
	if !ok {
		qps := r.qps
		if qps <= 0 {
			qps = 40
		}
		burst := r.burst
		if burst <= 0 {
			burst = 60
		}
		limiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
		r.limiters[key] = limiter
	}
	return limiter
}
