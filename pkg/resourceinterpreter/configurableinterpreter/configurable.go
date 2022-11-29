package configurableinterpreter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/executor"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// ConfigurableInterpreter interprets resources with resource interpreter customizations.
type ConfigurableInterpreter struct {
	// configManager caches all ResourceInterpreterCustomizations.
	configManager configmanager.ConfigManager
	executors     []executor.Interface
}

// NewConfigurableInterpreter builds a new interpreter by registering the
// event handler to the provided informer instance.
func NewConfigurableInterpreter(informer genericmanager.SingleClusterInformerManager) *ConfigurableInterpreter {
	return &ConfigurableInterpreter{
		configManager: configmanager.NewInterpreterConfigManager(informer),
		executors: []executor.Interface{
			executor.NewLuaExecutor(),
		},
	}
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (c *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	interpreter := c.configManager.GetInterpreter(kind)
	if interpreter != nil {
		switch operationType {
		case configv1alpha1.InterpreterOperationAggregateStatus:
			return interpreter.StatusAggregation != nil
		case configv1alpha1.InterpreterOperationInterpretHealth:
			return interpreter.HealthInterpretation != nil
		case configv1alpha1.InterpreterOperationInterpretDependency:
			return interpreter.DependencyInterpretation != nil
		case configv1alpha1.InterpreterOperationInterpretReplica:
			return interpreter.ReplicaResource != nil
		case configv1alpha1.InterpreterOperationInterpretStatus:
			return interpreter.StatusReflection != nil
		case configv1alpha1.InterpreterOperationRetain:
			return interpreter.Retention != nil
		case configv1alpha1.InterpreterOperationReviseReplica:
			return interpreter.ReplicaRevision != nil
		}
	}
	return false
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (replicas int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(object.GroupVersionKind())
	if interpreter == nil || interpreter.ReplicaResource == nil {
		return
	}

	for _, exec := range c.executors {
		replicas, requires, enabled, err = exec.GetReplicas(interpreter.ReplicaResource, object)
		if enabled || err != nil {
			return
		}
	}
	return
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(object.GroupVersionKind())
	if interpreter == nil || interpreter.ReplicaRevision == nil {
		return
	}

	for _, exec := range c.executors {
		revised, enabled, err = exec.ReviseReplica(interpreter.ReplicaRevision, object, replica)
		if enabled || err != nil {
			return
		}
	}
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(desired.GroupVersionKind())
	if interpreter == nil || interpreter.Retention == nil {
		return
	}

	for _, exec := range c.executors {
		retained, enabled, err = exec.Retain(interpreter.Retention, desired, observed)
		if enabled || err != nil {
			return
		}
	}
	return
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (status *unstructured.Unstructured, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(object.GroupVersionKind())
	if interpreter == nil || interpreter.StatusAggregation == nil {
		return
	}

	for _, exec := range c.executors {
		status, enabled, err = exec.AggregateStatus(interpreter.StatusAggregation, object, aggregatedStatusItems)
		if enabled || err != nil {
			return
		}
	}
	return
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(object.GroupVersionKind())
	if interpreter == nil || interpreter.DependencyInterpretation == nil {
		return
	}

	for _, exec := range c.executors {
		dependencies, enabled, err = exec.GetDependencies(interpreter.DependencyInterpretation, object)
		if enabled || err != nil {
			return
		}
	}
	return
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(object.GroupVersionKind())
	if interpreter == nil || interpreter.StatusReflection == nil {
		return
	}

	for _, exec := range c.executors {
		status, enabled, err = exec.ReflectStatus(interpreter.StatusReflection, object)
		if enabled || err != nil {
			return
		}
	}
	return
}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (health bool, enabled bool, err error) {
	interpreter := c.configManager.GetInterpreter(object.GroupVersionKind())
	if interpreter == nil || interpreter.HealthInterpretation == nil {
		return
	}

	for _, exec := range c.executors {
		health, enabled, err = exec.InterpretHealth(interpreter.HealthInterpretation, object)
		if enabled || err != nil {
			return
		}
	}
	return
}

// LoadConfig loads and stores rules from customizations
func (c *ConfigurableInterpreter) LoadConfig(customizations []*configv1alpha1.ResourceInterpreterCustomization) {
	c.configManager.LoadConfig(customizations)
}
