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

package plugins

import (
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/apienablement"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clustereviction"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusterlocality"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/spreadconstraint"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/tainttoleration"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
)

// NewInTreeRegistry builds the registry with all the in-tree plugins.
func NewInTreeRegistry() runtime.Registry {
	return runtime.Registry{
		apienablement.Name:    apienablement.New,
		tainttoleration.Name:  tainttoleration.New,
		clusteraffinity.Name:  clusteraffinity.New,
		spreadconstraint.Name: spreadconstraint.New,
		clusterlocality.Name:  clusterlocality.New,
		clustereviction.Name:  clustereviction.New,
	}
}
