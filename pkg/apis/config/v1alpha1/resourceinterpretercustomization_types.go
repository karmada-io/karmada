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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ResourceKindResourceInterpreterCustomization is kind name of ResourceInterpreterCustomization.
	ResourceKindResourceInterpreterCustomization = "ResourceInterpreterCustomization"
	// ResourceSingularResourceInterpreterCustomization is singular name of ResourceInterpreterCustomization.
	ResourceSingularResourceInterpreterCustomization = "resourceinterpretercustomization"
	// ResourcePluralResourceInterpreterCustomization is plural name of ResourceInterpreterCustomization.
	ResourcePluralResourceInterpreterCustomization = "resourceinterpretercustomizations"
	// ResourceNamespaceScopedResourceInterpreterCustomization indicates if ResourceInterpreterCustomization is NamespaceScoped.
	ResourceNamespaceScopedResourceInterpreterCustomization = false
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=resourceinterpretercustomizations,scope="Cluster",shortName=ric,categories={karmada-io}
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:JSONPath=`.spec.target.apiVersion`,name="TARGET-API-VERSION",type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.target.kind`,name="TARGET-KIND",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="AGE",type=date

// ResourceInterpreterCustomization describes the configuration of a specific
// resource for Karmada to get the structure.
// It has higher precedence than the default interpreter and the interpreter
// webhook.
type ResourceInterpreterCustomization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec describes the configuration in detail.
	// +required
	Spec ResourceInterpreterCustomizationSpec `json:"spec"`
}

// ResourceInterpreterCustomizationSpec describes the configuration in detail.
type ResourceInterpreterCustomizationSpec struct {
	// CustomizationTarget represents the resource type that the customization applies to.
	// +required
	Target CustomizationTarget `json:"target"`

	// Customizations describe the interpretation rules.
	// +required
	Customizations CustomizationRules `json:"customizations"`
}

// CustomizationTarget represents the resource type that the customization applies to.
type CustomizationTarget struct {
	// APIVersion represents the API version of the target resource.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of target resources.
	// +required
	Kind string `json:"kind"`
}

// CustomizationRules describes the interpretation rules.
type CustomizationRules struct {
	// Retention describes the desired behavior that Karmada should react on
	// the changes made by member cluster components. This avoids system
	// running into a meaningless loop that Karmada resource controller and
	// the member cluster component continually applying opposite values of a field.
	// For example, the "replicas" of Deployment might be changed by the HPA
	// controller on member cluster. In this case, Karmada should retain the "replicas"
	// and not try to change it.
	// +optional
	Retention *LocalValueRetention `json:"retention,omitempty"`

	// ReplicaResource describes the rules for Karmada to discover the resource's
	// replica as well as resource requirements.
	// It would be useful for those CRD resources that declare workload types like
	// Deployment.
	// It is usually not needed for Kubernetes native resources(Deployment, Job) as
	// Karmada knows how to discover info from them. But if it is set, the built-in
	// discovery rules will be ignored.
	// +optional
	ReplicaResource *ReplicaResourceRequirement `json:"replicaResource,omitempty"`

	// ReplicaRevision describes the rules for Karmada to revise the resource's replica.
	// It would be useful for those CRD resources that declare workload types like
	// Deployment.
	// It is usually not needed for Kubernetes native resources(Deployment, Job) as
	// Karmada knows how to revise replicas for them. But if it is set, the built-in
	// revision rules will be ignored.
	// +optional
	ReplicaRevision *ReplicaRevision `json:"replicaRevision,omitempty"`

	// StatusReflection describes the rules for Karmada to pick the resource's status.
	// Karmada provides built-in rules for several standard Kubernetes types, see:
	// https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter/#interpretstatus
	// If StatusReflection is set, the built-in rules will be ignored.
	// +optional
	StatusReflection *StatusReflection `json:"statusReflection,omitempty"`

	// StatusAggregation describes the rules for Karmada to aggregate status
	// collected from member clusters to resource template.
	// Karmada provides built-in rules for several standard Kubernetes types, see:
	// https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter/#aggregatestatus
	// If StatusAggregation is set, the built-in rules will be ignored.
	// +optional
	StatusAggregation *StatusAggregation `json:"statusAggregation,omitempty"`

	// HealthInterpretation describes the health assessment rules by which Karmada
	// can assess the health state of the resource type.
	// +optional
	HealthInterpretation *HealthInterpretation `json:"healthInterpretation,omitempty"`

	// DependencyInterpretation describes the rules for Karmada to analyze the
	// dependent resources.
	// Karmada provides built-in rules for several standard Kubernetes types, see:
	// https://karmada.io/docs/userguide/globalview/customizing-resource-interpreter/#interpretdependency
	// If DependencyInterpretation is set, the built-in rules will be ignored.
	// +optional
	DependencyInterpretation *DependencyInterpretation `json:"dependencyInterpretation,omitempty"`
}

// LocalValueRetention holds the scripts for retention.
// Now only supports Lua.
type LocalValueRetention struct {
	// LuaScript holds the Lua script that is used to retain runtime values
	// to the desired specification.
	//
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function Retain(desiredObj, observedObj)
	//           desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo
	//           return desiredObj
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - desiredObj: the object represents the configuration to be applied
	//       to the member cluster.
	//   - observedObj: the object represents the configuration that is observed
	//       from a specific member cluster.
	//
	// The returned object should be a retained configuration which will be
	// applied to member cluster eventually.
	// +required
	LuaScript string `json:"luaScript"`
}

// ReplicaResourceRequirement holds the scripts for getting the desired replicas
// as well as the resource requirement of each replica.
type ReplicaResourceRequirement struct {
	// LuaScript holds the Lua script that is used to discover the resource's
	// replica as well as resource requirements
	//
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function GetReplicas(desiredObj)
	//           replica = desiredObj.spec.replicas
	//           requirement = {}
	//           requirement.nodeClaim = {}
	//           requirement.nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector
	//           requirement.nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations
	//           requirement.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits
	//           return replica, requirement
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - desiredObj: the object represents the configuration to be applied
	//       to the member cluster.
	//
	// The function expects two return values:
	//   - replica: the declared replica number
	//   - requirement: the resource required by each replica expressed with a
	//       ResourceBindingSpec.ReplicaRequirements.
	// The returned values will be set into a ResourceBinding or ClusterResourceBinding.
	// +required
	LuaScript string `json:"luaScript"`
}

// ReplicaRevision holds the scripts for revising the desired replicas.
type ReplicaRevision struct {
	// LuaScript holds the Lua script that is used to revise replicas in the desired specification.
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function ReviseReplica(desiredObj, desiredReplica)
	//           desiredObj.spec.replicas = desiredReplica
	//           return desiredObj
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - desiredObj: the object represents the configuration to be applied
	//       to the member cluster.
	//   - desiredReplica: the replica number should be applied with.
	//
	// The returned object should be a revised configuration which will be
	// applied to member cluster eventually.
	// +required
	LuaScript string `json:"luaScript"`
}

// StatusReflection holds the scripts for getting the status.
type StatusReflection struct {
	// LuaScript holds the Lua script that is used to get the status from the observed specification.
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function ReflectStatus(observedObj)
	//           status = {}
	//           status.readyReplicas = observedObj.status.observedObj
	//           return status
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - observedObj: the object represents the configuration that is observed
	//       from a specific member cluster.
	//
	// The returned status could be the whole status or part of it and will
	// be set into both Work and ResourceBinding(ClusterResourceBinding).
	// +required
	LuaScript string `json:"luaScript"`
}

// StatusAggregation holds the scripts for aggregating several decentralized statuses.
type StatusAggregation struct {
	// LuaScript holds the Lua script that is used to aggregate decentralized statuses
	// to the desired specification.
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function AggregateStatus(desiredObj, statusItems)
	//           for i = 1, #statusItems do
	//               desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + items[i].readyReplicas
	//           end
	//           return desiredObj
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - desiredObj: the object represents a resource template.
	//   - statusItems: the slice of status expressed with AggregatedStatusItem.
	//
	// The returned object should be a whole object with status aggregated.
	//
	// +required
	LuaScript string `json:"luaScript"`
}

// HealthInterpretation holds the rules for interpreting the health state of a specific resource.
type HealthInterpretation struct {
	// LuaScript holds the Lua script that is used to assess the health state of
	// a specific resource.
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function InterpretHealth(observedObj)
	//           if observedObj.status.readyReplicas == observedObj.spec.replicas then
	//               return true
	//           end
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - observedObj: the object represents the configuration that is observed
	//       from a specific member cluster.
	//
	// The returned boolean value indicates the health status.
	//
	// +required
	LuaScript string `json:"luaScript"`
}

// DependencyInterpretation holds the rules for interpreting the dependent resources
// of a specific resources.
type DependencyInterpretation struct {
	// LuaScript holds the Lua script that is used to interpret the dependencies of
	// a specific resource.
	// The script should implement a function as follows:
	//
	// ```
	//   luaScript: >
	//       function GetDependencies(desiredObj)
	//           dependencies = {}
	//           serviceAccountName = desiredObj.spec.template.spec.serviceAccountName
	//           if serviceAccountName ~= nil and serviceAccountName ~= "default" then
	//               dependency = {}
	//               dependency.apiVersion = "v1"
	//               dependency.kind = "ServiceAccount"
	//               dependency.name = serviceAccountName
	//               dependency.namespace = desiredObj.metadata.namespace
	//               dependencies[1] = dependency
	//           end
	//           return dependencies
	//       end
	// ```
	//
	// The content of the LuaScript needs to be a whole function including both
	// declaration and implementation.
	//
	// The parameters will be supplied by the system:
	//   - desiredObj: the object represents the configuration to be applied
	//       to the member cluster.
	//
	// The returned value should be expressed by a slice of DependentObjectReference.
	// +required
	LuaScript string `json:"luaScript"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceInterpreterCustomizationList contains a list of ResourceInterpreterCustomization.
type ResourceInterpreterCustomizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceInterpreterCustomization `json:"items"`
}
