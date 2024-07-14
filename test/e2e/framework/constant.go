/*
Copyright 2021 The Karmada Authors.

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
