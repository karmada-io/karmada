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

package configmanager

import (
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// CustomConfiguration provides base information about custom interpreter configuration
type CustomConfiguration interface {
	Merge(rules configv1alpha1.CustomizationRules)
}

// LuaScriptAccessor provides a common interface to get custom interpreter lua script
type LuaScriptAccessor interface {
	CustomConfiguration

	GetRetentionLuaScript() string
	GetReplicaResourceLuaScript() string
	GetReplicaRevisionLuaScript() string
	GetStatusReflectionLuaScript() string
	GetStatusAggregationLuaScript() string
	GetHealthInterpretationLuaScript() string
	GetDependencyInterpretationLuaScripts() []string
}

// CustomAccessor provides a common interface to get custom interpreter configuration.
type CustomAccessor interface {
	LuaScriptAccessor
}

type resourceCustomAccessor struct {
	retention                 *configv1alpha1.LocalValueRetention
	replicaResource           *configv1alpha1.ReplicaResourceRequirement
	replicaRevision           *configv1alpha1.ReplicaRevision
	statusReflection          *configv1alpha1.StatusReflection
	statusAggregation         *configv1alpha1.StatusAggregation
	healthInterpretation      *configv1alpha1.HealthInterpretation
	dependencyInterpretations []*configv1alpha1.DependencyInterpretation
}

// NewResourceCustomAccessor creates an accessor for resource interpreter customization.
func NewResourceCustomAccessor() CustomAccessor {
	return &resourceCustomAccessor{}
}

// Merge merges the given CustomizationRules with the current rules, ignore if duplicates occur.
func (a *resourceCustomAccessor) Merge(rules configv1alpha1.CustomizationRules) {
	if rules.Retention != nil {
		a.setRetain(rules.Retention)
	}
	if rules.ReplicaResource != nil {
		a.setReplicaResource(rules.ReplicaResource)
	}
	if rules.ReplicaRevision != nil {
		a.setReplicaRevision(rules.ReplicaRevision)
	}
	if rules.StatusReflection != nil {
		a.setStatusReflection(rules.StatusReflection)
	}
	if rules.StatusAggregation != nil {
		a.setStatusAggregation(rules.StatusAggregation)
	}
	if rules.HealthInterpretation != nil {
		a.setHealthInterpretation(rules.HealthInterpretation)
	}
	if rules.DependencyInterpretation != nil {
		a.appendDependencyInterpretation(rules.DependencyInterpretation)
	}
}

func (a *resourceCustomAccessor) GetRetentionLuaScript() string {
	if a.retention == nil {
		return ""
	}
	return a.retention.LuaScript
}

func (a *resourceCustomAccessor) GetReplicaResourceLuaScript() string {
	if a.replicaResource == nil {
		return ""
	}
	return a.replicaResource.LuaScript
}

func (a *resourceCustomAccessor) GetReplicaRevisionLuaScript() string {
	if a.replicaRevision == nil {
		return ""
	}
	return a.replicaRevision.LuaScript
}

func (a *resourceCustomAccessor) GetStatusReflectionLuaScript() string {
	if a.statusReflection == nil {
		return ""
	}
	return a.statusReflection.LuaScript
}

func (a *resourceCustomAccessor) GetStatusAggregationLuaScript() string {
	if a.statusAggregation == nil {
		return ""
	}
	return a.statusAggregation.LuaScript
}

func (a *resourceCustomAccessor) GetHealthInterpretationLuaScript() string {
	if a.healthInterpretation == nil {
		return ""
	}
	return a.healthInterpretation.LuaScript
}

func (a *resourceCustomAccessor) GetDependencyInterpretationLuaScripts() []string {
	if a.dependencyInterpretations == nil {
		return nil
	}

	var scripts []string
	for _, interpretation := range a.dependencyInterpretations {
		if interpretation.LuaScript != "" {
			scripts = append(scripts, interpretation.LuaScript)
		}
	}
	return scripts
}

func (a *resourceCustomAccessor) setRetain(retention *configv1alpha1.LocalValueRetention) {
	if a.retention == nil {
		a.retention = retention
		return
	}

	if retention.LuaScript != "" && a.retention.LuaScript == "" {
		a.retention.LuaScript = retention.LuaScript
	}
}

func (a *resourceCustomAccessor) setReplicaResource(replicaResource *configv1alpha1.ReplicaResourceRequirement) {
	if a.replicaResource == nil {
		a.replicaResource = replicaResource
		return
	}

	if replicaResource.LuaScript != "" && a.replicaResource.LuaScript == "" {
		a.replicaResource.LuaScript = replicaResource.LuaScript
	}
}

func (a *resourceCustomAccessor) setReplicaRevision(replicaRevision *configv1alpha1.ReplicaRevision) {
	if a.replicaRevision == nil {
		a.replicaRevision = replicaRevision
		return
	}

	if replicaRevision.LuaScript != "" && a.replicaRevision.LuaScript == "" {
		a.replicaRevision.LuaScript = replicaRevision.LuaScript
	}
}

func (a *resourceCustomAccessor) setStatusReflection(statusReflection *configv1alpha1.StatusReflection) {
	if a.statusReflection == nil {
		a.statusReflection = statusReflection
		return
	}

	if statusReflection.LuaScript != "" && a.statusReflection.LuaScript == "" {
		a.statusReflection.LuaScript = statusReflection.LuaScript
	}
}

func (a *resourceCustomAccessor) setStatusAggregation(statusAggregation *configv1alpha1.StatusAggregation) {
	if a.statusAggregation == nil {
		a.statusAggregation = statusAggregation
		return
	}

	if statusAggregation.LuaScript != "" && a.statusAggregation.LuaScript == "" {
		a.statusAggregation.LuaScript = statusAggregation.LuaScript
	}
}

func (a *resourceCustomAccessor) setHealthInterpretation(healthInterpretation *configv1alpha1.HealthInterpretation) {
	if a.healthInterpretation == nil {
		a.healthInterpretation = healthInterpretation
		return
	}

	if healthInterpretation.LuaScript != "" && a.healthInterpretation.LuaScript == "" {
		a.healthInterpretation.LuaScript = healthInterpretation.LuaScript
	}
}

func (a *resourceCustomAccessor) appendDependencyInterpretation(dependencyInterpretation *configv1alpha1.DependencyInterpretation) {
	a.dependencyInterpretations = append(a.dependencyInterpretations, dependencyInterpretation)
}
