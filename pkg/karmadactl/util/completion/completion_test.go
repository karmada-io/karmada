/*
Copyright 2026 The Karmada Authors.

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

package completion

import (
	"testing"
	"time"

	"k8s.io/client-go/rest"

	utiltesting "github.com/karmada-io/karmada/pkg/karmadactl/util/testing"
)

func TestTimeoutRESTClientGetterToRESTConfig(t *testing.T) {
	tests := []struct {
		name      string
		baseCfg   *rest.Config
		wantTimer time.Duration
	}{
		{
			name:      "set timeout on empty config",
			baseCfg:   &rest.Config{},
			wantTimer: 5 * time.Second,
		},
		{
			name:      "override existing timeout",
			baseCfg:   &rest.Config{Timeout: 2 * time.Second},
			wantTimer: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := utiltesting.NewTestFactory()
			base.ClientConfigVal = rest.CopyConfig(tt.baseCfg)
			defer base.Cleanup()

			getter := &timeoutRESTClientGetter{RESTClientGetter: base, timeout: 5 * time.Second}

			cfg, err := getter.ToRESTConfig()
			if err != nil {
				t.Fatalf("ToRESTConfig() unexpected error: %v", err)
			}
			if cfg.Timeout != tt.wantTimer {
				t.Fatalf("expected timeout %v, got %v", tt.wantTimer, cfg.Timeout)
			}
		})
	}
}
