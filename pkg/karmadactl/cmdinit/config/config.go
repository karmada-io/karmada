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

package config

import (
	"fmt"
	"os"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
)

// LoadInitConfiguration loads the InitConfiguration from the specified file path.
// It delegates the actual loading to the loadInitConfigurationFromFile function.
func LoadInitConfiguration(cfgPath string) (*KarmadaInitConfig, error) {
	var config *KarmadaInitConfig
	var err error

	config, err = loadInitConfigurationFromFile(cfgPath)

	return config, err
}

// loadInitConfigurationFromFile reads the file at the specified path and converts it into an InitConfiguration.
// It reads the file contents and then converts the bytes to an InitConfiguration.
func loadInitConfigurationFromFile(cfgPath string) (*KarmadaInitConfig, error) {
	klog.V(1).Infof("loading configuration from %q", cfgPath)

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read config from %q: %v", cfgPath, err)
	}
	gvkmap, err := ParseGVKYamlMap(b)
	if err != nil {
		return nil, err
	}

	return documentMapToInitConfiguration(gvkmap)
}

// ParseGVKYamlMap parses a single YAML document into a map of GroupVersionKind to byte slices.
// This function is a simplified version that handles only a single YAML document.
func ParseGVKYamlMap(yamlBytes []byte) (map[schema.GroupVersionKind][]byte, error) {
	gvkmap := make(map[schema.GroupVersionKind][]byte)

	gvk, err := yamlserializer.DefaultMetaFactory.Interpret(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to interpret YAML document: %w", err)
	}
	if len(gvk.Group) == 0 || len(gvk.Version) == 0 || len(gvk.Kind) == 0 {
		return nil, fmt.Errorf("invalid configuration for GroupVersionKind %+v: kind and apiVersion is mandatory information that must be specified", gvk)
	}
	gvkmap[*gvk] = yamlBytes

	return gvkmap, nil
}

// documentMapToInitConfiguration processes a map of GroupVersionKind to byte slices to extract the InitConfiguration.
// It iterates over the map, checking for the "InitConfiguration" kind, group, and version, and unmarshals its content into an InitConfiguration object.
func documentMapToInitConfiguration(gvkmap map[schema.GroupVersionKind][]byte) (*KarmadaInitConfig, error) {
	var initcfg *KarmadaInitConfig

	gvks := make([]schema.GroupVersionKind, 0, len(gvkmap))
	for gvk := range gvkmap {
		gvks = append(gvks, gvk)
	}
	sort.Slice(gvks, func(i, j int) bool {
		return gvks[i].String() < gvks[j].String()
	})

	for _, gvk := range gvks {
		fileContent := gvkmap[gvk]
		if gvk.Kind == "KarmadaInitConfig" {
			if gvk.Group != GroupName || gvk.Version != SchemeGroupVersion.Version {
				return nil, fmt.Errorf("invalid Group or Version: expected group %q and version %q, but got group %q and version %q", GroupName, SchemeGroupVersion.Version, gvk.Group, gvk.Version)
			}
			initcfg = &KarmadaInitConfig{}
			if err := yaml.Unmarshal(fileContent, initcfg); err != nil {
				return nil, err
			}
		}
	}

	if initcfg == nil {
		return nil, fmt.Errorf("no KarmadaInitConfig kind was found in the YAML file")
	}

	return initcfg, nil
}
