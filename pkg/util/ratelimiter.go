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

var defaultRateLimiterGetter = &RateLimiterGetter{}

// GetRateLimiterGetter returns a RateLimiterGetter.
func GetRateLimiterGetter() *RateLimiterGetter {
	return defaultRateLimiterGetter
}

// RateLimiterGetter is a struct to get rate limiter.
type RateLimiterGetter struct {
	limiters map[string]flowcontrol.RateLimiter
	mu       sync.Mutex
	qps      float32
	burst    int
}

// SetLimits sets the qps and burst.
func (r *RateLimiterGetter) SetLimits(qps float32, burst int) *RateLimiterGetter {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.qps = qps
	r.burst = burst
	return r
}

// GetRateLimiter gets rate limiter by key.
func (r *RateLimiterGetter) GetRateLimiter(key string) flowcontrol.RateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limiters == nil {
		r.limiters = make(map[string]flowcontrol.RateLimiter)
	}
	limiter, ok := r.limiters[key]
	if !ok {
		qps := r.qps
		if qps <= 0 {
			qps = 5
		}
		burst := r.burst
		if burst <= 0 {
			burst = 10
		}
		limiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
		r.limiters[key] = limiter
	}
	return limiter
}
