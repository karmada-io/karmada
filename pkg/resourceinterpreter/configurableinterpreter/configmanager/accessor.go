package configmanager

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// CustomConfiguration provides base information about custom interpreter configuration
type CustomConfiguration interface {
	Name() string
	TargetResource() schema.GroupVersionKind
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
	GetDependencyInterpretationLuaScript() string
}

// CustomAccessor provides a common interface to get custom interpreter configuration.
type CustomAccessor interface {
	LuaScriptAccessor
}

type resourceCustomAccessor struct {
	retention                *configv1alpha1.LocalValueRetention
	replicaResource          *configv1alpha1.ReplicaResourceRequirement
	replicaRevision          *configv1alpha1.ReplicaRevision
	statusReflection         *configv1alpha1.StatusReflection
	statusAggregation        *configv1alpha1.StatusAggregation
	healthInterpretation     *configv1alpha1.HealthInterpretation
	dependencyInterpretation *configv1alpha1.DependencyInterpretation
	configurationName        string
	configurationTargetGVK   schema.GroupVersionKind
}

// NewResourceCustomAccessorAccessor creates an accessor for resource interpreter customization
func NewResourceCustomAccessorAccessor(customization *configv1alpha1.ResourceInterpreterCustomization) CustomAccessor {
	return &resourceCustomAccessor{
		retention:                customization.Spec.Customizations.Retention,
		replicaResource:          customization.Spec.Customizations.ReplicaResource,
		replicaRevision:          customization.Spec.Customizations.ReplicaRevision,
		statusReflection:         customization.Spec.Customizations.StatusReflection,
		statusAggregation:        customization.Spec.Customizations.StatusAggregation,
		healthInterpretation:     customization.Spec.Customizations.HealthInterpretation,
		dependencyInterpretation: customization.Spec.Customizations.DependencyInterpretation,
		configurationName:        customization.Name,
		configurationTargetGVK:   schema.FromAPIVersionAndKind(customization.Spec.Target.APIVersion, customization.Spec.Target.Kind),
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

func (a *resourceCustomAccessor) GetDependencyInterpretationLuaScript() string {
	if a.dependencyInterpretation == nil {
		return ""
	}
	return a.dependencyInterpretation.LuaScript
}

func (a *resourceCustomAccessor) Name() string {
	return a.configurationName
}

func (a *resourceCustomAccessor) TargetResource() schema.GroupVersionKind {
	return a.configurationTargetGVK
}
