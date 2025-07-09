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

package config

import (
	"github.com/spf13/pflag"
)

// EvictionQueueOptions holds the options that control the behavior of the graceful eviction queue based on the overall health of the clusters.
type EvictionQueueOptions struct {
	// ResourceEvictionRate is the number of resources to be evicted per second.
	// This is the default rate when the system is considered healthy.
	ResourceEvictionRate float32
	// SecondaryResourceEvictionRate is the secondary resource eviction rate.
	// When the number of cluster failures in the Karmada instance exceeds the unhealthy-cluster-threshold,
	// the resource eviction rate will be reduced to this secondary level.
	SecondaryResourceEvictionRate float32
	// UnhealthyClusterThreshold is the threshold of unhealthy clusters.
	// If the ratio of unhealthy clusters to total clusters exceeds this threshold, the Karmada instance is considered unhealthy,
	// and the eviction rate will be downgraded to the secondary rate.
	UnhealthyClusterThreshold float32
	// LargeClusterNumThreshold is the threshold for a large-scale Karmada instance.
	// When the number of clusters in the instance exceeds this threshold and the instance is unhealthy,
	// the eviction rate is downgraded. For smaller instances that are unhealthy, eviction might be halted completely.
	LargeClusterNumThreshold int
}

// AddFlags adds flags for the EvictionQueueOptions to the specified FlagSet.
func (o *EvictionQueueOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.Float32Var(&o.ResourceEvictionRate, "resource-eviction-rate", 0.5, "The number of resources to be evicted per second.")
	fs.Float32Var(&o.SecondaryResourceEvictionRate, "secondary-resource-eviction-rate", 0.1, "The secondary resource eviction rate when the Karmada instance is unhealthy.")
	fs.Float32Var(&o.UnhealthyClusterThreshold, "unhealthy-cluster-threshold", 0.55, "The unhealthy threshold of the cluster, if the ratio of unhealthy clusters to total clusters exceeds thisthreshold, the Karmada instance is considered unhealthy.")
	fs.IntVar(&o.LargeClusterNumThreshold, "large-cluster-num-threshold", 10, "The large-scale threshold of the Karmada instance. When the number of clusters in a large-scale federation exceedsthis threshold and the federation is unhealthy, the resource eviction rate will be reduced; otherwise, the eviction will be stopped.")
}
