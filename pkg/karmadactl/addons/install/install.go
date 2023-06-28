package install

import (
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/descheduler"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/estimator"
	addonsinit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/metricsadapter"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/search"
)

// Install intall the karmada addons process in Addons
func Install() {
	addonsinit.Addons["karmada-descheduler"] = descheduler.AddonDescheduler
	addonsinit.Addons["karmada-metrics-adapter"] = metricsadapter.AddonMetricsAdapter
	addonsinit.Addons["karmada-scheduler-estimator"] = estimator.AddonEstimator
	addonsinit.Addons["karmada-search"] = search.AddonSearch
}
