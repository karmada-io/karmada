/*
Copyright 2024 The Karmada Authors.

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
	"reflect"
	"testing"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons/descheduler"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/estimator"
	addonsinit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/metricsadapter"
	"github.com/karmada-io/karmada/pkg/karmadactl/addons/search"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestInstall(t *testing.T) {
	// Call the function to install addons.
	Install()

	tests := []struct {
		name          string
		key           string
		expectedAddon interface{}
	}{
		{
			name:          "Install_WithKarmadaDeschedulerAddon_Installed",
			key:           names.KarmadaDeschedulerComponentName,
			expectedAddon: descheduler.AddonDescheduler,
		},
		{
			name:          "Install_WithKarmadaMetricsAdapterAddon_Installed",
			key:           names.KarmadaMetricsAdapterComponentName,
			expectedAddon: metricsadapter.AddonMetricsAdapter,
		},
		{
			name:          "Install_WithKarmadaSchedulerEstimatorAddon_Installed",
			key:           names.KarmadaSchedulerEstimatorComponentName,
			expectedAddon: estimator.AddonEstimator,
		},
		{
			name:          "Install_WithKarmadaSearchAddon_Installed",
			key:           names.KarmadaSearchComponentName,
			expectedAddon: search.AddonSearch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualAddon, exists := addonsinit.Addons[tt.key]
			if !exists {
				t.Fatalf("Expected addon %s to be registered, but it was not found", tt.key)
			}
			if !reflect.DeepEqual(actualAddon, tt.expectedAddon) {
				t.Errorf("Expected addon %s to be %v, but got %v", tt.key, tt.expectedAddon, actualAddon)
			}
		})
	}
}
