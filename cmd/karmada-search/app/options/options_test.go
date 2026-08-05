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

package options

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	basecompatibility "k8s.io/component-base/compatibility"

	"github.com/karmada-io/karmada/pkg/features"
)

func TestFeatureGateFlagUsesIsolatedKubeFeatureGate(t *testing.T) {
	for range 2 {
		o := NewOptions()
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		o.AddFlags(fs)

		flag := fs.Lookup("feature-gates")
		if flag == nil {
			t.Fatal("feature-gates flag is not registered")
		}
		if flag.Value.Type() != "colonSeparatedMultimapStringString" {
			t.Fatalf("unexpected feature-gates flag type: %s", flag.Value.Type())
		}

		usage := flag.Usage
		if !strings.Contains(usage, "kube:RawDynamicInformer=true|false") {
			t.Fatalf("feature-gates flag does not contain RawDynamicInformer: %s", usage)
		}
		if !strings.Contains(usage, "kube:APIServerIdentity=true|false") {
			t.Fatalf("feature-gates flag does not contain kube native features: %s", usage)
		}
	}
}

func TestNewOptionsWorksAfterDefaultFeatureGateFlagRegistration(t *testing.T) {
	// Swagger generation binds the global kube feature gate to a flag set before
	// karmada-search options are built.
	utilfeature.DefaultMutableFeatureGate.AddFlag(pflag.NewFlagSet("default-kube-gate", pflag.ContinueOnError))

	o := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	o.AddFlags(fs)

	if err := fs.Parse([]string{"--feature-gates=RawDynamicInformer=false"}); err != nil {
		t.Fatalf("failed to parse RawDynamicInformer after default feature gate flag registration: %v", err)
	}
	if err := o.Complete(); err != nil {
		t.Fatalf("failed to complete options after default feature gate flag registration: %v", err)
	}

	kubeFeatureGate := o.ServerRunOptions.ComponentGlobalsRegistry.FeatureGateFor(basecompatibility.DefaultKubeComponent)
	if kubeFeatureGate.Enabled(features.RawDynamicInformer) {
		t.Fatal("RawDynamicInformer should be disabled by the isolated kube feature gate")
	}
}

func TestFeatureGateFlagKeepsKubeFeatureGatesCompatible(t *testing.T) {
	o := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	o.AddFlags(fs)

	if err := fs.Parse([]string{
		"--feature-gates=APIServerIdentity=false",
		"--feature-gates=RawDynamicInformer=false",
	}); err != nil {
		t.Fatalf("failed to parse feature-gates flag: %v", err)
	}
	if err := o.Complete(); err != nil {
		t.Fatalf("failed to complete options: %v", err)
	}

	kubeFeatureGate := o.ServerRunOptions.ComponentGlobalsRegistry.FeatureGateFor(basecompatibility.DefaultKubeComponent)
	if kubeFeatureGate.Enabled(genericfeatures.APIServerIdentity) {
		t.Fatal("APIServerIdentity should be disabled by the legacy unprefixed kube feature gate")
	}
	if kubeFeatureGate.Enabled(features.RawDynamicInformer) {
		t.Fatal("RawDynamicInformer should be disabled by the unprefixed kube feature gate")
	}
}
