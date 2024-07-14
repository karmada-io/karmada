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

package install

import (
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/descheduler"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/estimator"
	addonsinit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/metricsadapter"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/search"
)

// Install install the karmada addons process in Addons
func Install() {
	addonsinit.Addons["karmada-descheduler"] = descheduler.AddonDescheduler
	addonsinit.Addons["karmada-metrics-adapter"] = metricsadapter.AddonMetricsAdapter
	addonsinit.Addons["karmada-scheduler-estimator"] = estimator.AddonEstimator
	addonsinit.Addons["karmada-search"] = search.AddonSearch
}
