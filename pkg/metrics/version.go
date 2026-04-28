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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/karmada-io/karmada/pkg/version"
)

// NewBuildInfoCollector returns a collector that exports metrics about current version
// information.
func NewBuildInfoCollector() prometheus.Collector {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "karmada_build_info",
			Help: "Karmada build metadata exposed as labels with a constant value of 1.",
			ConstLabels: prometheus.Labels{
				"git_version":      version.Get().GitVersion,
				"git_commit":       version.Get().GitCommit,
				"git_short_commit": version.Get().GitShortCommit,
				"git_tree_state":   version.Get().GitTreeState,
				"build_date":       version.Get().BuildDate,
				"go_version":       version.Get().GoVersion,
				"compiler":         version.Get().Compiler,
				"platform":         version.Get().Platform,
			},
		},
		func() float64 { return 1 },
	)
}
