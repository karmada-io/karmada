package ratelimiter

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// Options are options for rate limiter.
type Options struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	QPS        int
	BucketSize int
}

// DefaultControllerRateLimiter provide a default rate limiter for controller, and users can tune it by corresponding flags.
func DefaultControllerRateLimiter(opts Options) workqueue.RateLimiter {
	// set defaults
	if opts.BaseDelay <= 0 {
		opts.BaseDelay = 5 * time.Millisecond
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 1000 * time.Second
	}
	if opts.QPS <= 0 {
		opts.QPS = 10
	}
	if opts.BucketSize <= 0 {
		opts.BucketSize = 100
	}
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(opts.BaseDelay, opts.MaxDelay),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(opts.QPS), opts.BucketSize)},
	)
}
