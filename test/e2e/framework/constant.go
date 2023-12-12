package framework

import "time"

const (
	// pollInterval defines the interval time for a poll operation.
	pollInterval = 5 * time.Second
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout = 420 * time.Second
	// metricsCreationDelay defines the maximum time metrics not yet available for pod.
	metricsCreationDelay = 2 * time.Minute
)
