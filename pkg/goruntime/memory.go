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

package goruntime

import (
	"fmt"
	"runtime/debug"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"k8s.io/klog/v2"
)

// SetMemLimit to auto setting memory limit
func SetMemLimit(memlimitRatio float64) {
	if memlimitRatio >= 1.0 {
		memlimitRatio = 1.0
	} else if memlimitRatio <= 0.0 {
		memlimitRatio = 0.0
	}

	// the memlimitRatio argument to 0, effectively disabling auto memory limit for all users.
	if memlimitRatio == 0.0 {
		return
	}

	if _, err := memlimit.SetGoMemLimitWithOpts(
		memlimit.WithRatio(memlimitRatio),
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
	); err != nil {
		klog.Warning("Failed to set GOMEMLIMIT automatically", "component", "automemlimit", "err", err)
	}

	klog.Info(fmt.Sprintf("GOMEMLIMIT set to %d", debug.SetMemoryLimit(-1)))
}
