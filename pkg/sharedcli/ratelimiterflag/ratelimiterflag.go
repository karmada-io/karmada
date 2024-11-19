/*
Copyright 2022 The Karmada Authors.

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

package ratelimiterflag

import (
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// Options are options for rate limiter.
type Options struct {
	// RateLimiterBaseDelay is the base delay for ItemExponentialFailureRateLimiter.
	RateLimiterBaseDelay time.Duration

	// RateLimiterMaxDelay is the max delay for ItemExponentialFailureRateLimiter.
	RateLimiterMaxDelay time.Duration

	// RateLimiterQPS is the qps for BucketRateLimiter
	RateLimiterQPS int

	// RateLimiterBucketSize is the bucket size for BucketRateLimiter
	RateLimiterBucketSize int
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.RateLimiterBaseDelay, "rate-limiter-base-delay", 5*time.Millisecond, "The base delay for rate limiter.")
	fs.DurationVar(&o.RateLimiterMaxDelay, "rate-limiter-max-delay", 1000*time.Second, "The max delay for rate limiter.")
	fs.IntVar(&o.RateLimiterQPS, "rate-limiter-qps", 10, "The QPS for rate limier.")
	fs.IntVar(&o.RateLimiterBucketSize, "rate-limiter-bucket-size", 100, "The bucket size for rate limier.")
}

// DefaultControllerRateLimiter provide a default rate limiter for controller, and users can tune it by corresponding flags.
func DefaultControllerRateLimiter[T comparable](opts Options) workqueue.TypedRateLimiter[T] {
	// set defaults
	if opts.RateLimiterBaseDelay <= 0 {
		opts.RateLimiterBaseDelay = 5 * time.Millisecond
	}
	if opts.RateLimiterMaxDelay <= 0 {
		opts.RateLimiterMaxDelay = 1000 * time.Second
	}
	if opts.RateLimiterQPS <= 0 {
		opts.RateLimiterQPS = 10
	}
	if opts.RateLimiterBucketSize <= 0 {
		opts.RateLimiterBucketSize = 100
	}

	return workqueue.NewTypedMaxOfRateLimiter[T](
		workqueue.NewTypedItemExponentialFailureRateLimiter[T](opts.RateLimiterBaseDelay, opts.RateLimiterMaxDelay),
		&workqueue.TypedBucketRateLimiter[T]{Limiter: rate.NewLimiter(rate.Limit(opts.RateLimiterQPS), opts.RateLimiterBucketSize)},
	)
}
