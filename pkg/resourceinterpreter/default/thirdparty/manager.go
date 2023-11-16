/*
Copyright 2023 The Karmada Authors.

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

package thirdparty

import (
	"bytes"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/thirdparty/resourcecustomizations"
)

const configurableInterpreterFile = "customizations.yaml"

// configManager collects the thirdparty resource interpreter customization.
type configManager struct {
	configuration atomic.Value
}

func (t *configManager) HasSynced() bool {
	return true
}

func (t *configManager) LoadConfig(customizations []*configv1alpha1.ResourceInterpreterCustomization) {
	sort.Slice(customizations, func(i, j int) bool {
		return customizations[i].Name < customizations[j].Name
	})

	accessors := make(map[schema.GroupVersionKind]configmanager.CustomAccessor)
	for _, config := range customizations {
		key := schema.FromAPIVersionAndKind(config.Spec.Target.APIVersion, config.Spec.Target.Kind)
		var ac configmanager.CustomAccessor
		var ok bool
		if ac, ok = accessors[key]; !ok {
			ac = configmanager.NewResourceCustomAccessor()
		}
		ac.Merge(config.Spec.Customizations)
		accessors[key] = ac
	}

	t.configuration.Store(accessors)
}

// CustomAccessors returns all cached configurations.
func (t *configManager) CustomAccessors() map[schema.GroupVersionKind]configmanager.CustomAccessor {
	return t.configuration.Load().(map[schema.GroupVersionKind]configmanager.CustomAccessor)
}

// NewThirdPartyConfigManager load third party resource in the cache.
func NewThirdPartyConfigManager() configmanager.ConfigManager {
	manager := &configManager{}
	manager.configuration.Store(make(map[schema.GroupVersionKind]configmanager.CustomAccessor))
	manager.loadThirdPartyConfig()
	return manager
}

func (t *configManager) loadThirdPartyConfig() {
	var configs []*configv1alpha1.ResourceInterpreterCustomization
	if err := fs.WalkDir(resourcecustomizations.Embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// cannot happen
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.Contains(d.Name(), "testdata") {
			return nil
		}

		if filepath.Base(d.Name()) != configurableInterpreterFile {
			return nil
		}
		data, err := fs.ReadFile(resourcecustomizations.Embedded, path)
		if err != nil {
			// cannot happen
			return err
		}
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		for {
			config := &configv1alpha1.ResourceInterpreterCustomization{}
			err = decoder.Decode(config)
			if err != nil {
				break
			}
			configs = append(configs, config)
		}
		if err != io.EOF {
			return err
		}
		return nil
	}); err != nil {
		klog.Warning(err, "failed to load third party resource")
		return
	}
	t.LoadConfig(configs)
}
