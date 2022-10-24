package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:storageversion

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
	// LuaScript holds the Lua script that is used to retain runtime values to the desired specification.
	// The script should implement a function as follows:
	//     luaScript: >
	//         function Retain(desiredObj, observedObj)
	//             desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo
	//             return desiredObj
	//         end
	//
	// LuaScript only holds the function body part, take the `desiredObj` and `observedObj` as the function
	// parameters or global variables, and finally returns a retained specification.
	// +required
	LuaScript string `json:"luaScript"`
}

// ReplicaResourceRequirement holds the scripts for getting the desired replicas
// as well as the resource requirement of each replica.
type ReplicaResourceRequirement struct {
	// LuaScript holds the Lua script that is used to discover the resource's
	// replica as well as resource requirements
	// The script should implement a function as follows:
	//     luaScript: >
	//         function GetReplicas(desiredObj)
	//             nodeClaim = {}
	//             resourceRequest = {}
	//             result = {}
	//
	//             result.replica = desiredObj.spec.replicas
	//             result.resourceRequest = desiredObj.spec.template.spec.containers[0].resources.limits
	//
	//             nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector
	//             nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations
	//             result.nodeClaim = nodeClaim
	//
	//             return result
	//         end
	//
	// LuaScript only holds the function body part, take the `desiredObj` as the function
	// parameters or global variable, and finally returns the replica number and required resources.
	// +required
	LuaScript string `json:"luaScript"`
}

// ReplicaRevision holds the scripts for revising the desired replicas.
type ReplicaRevision struct {
	// LuaScript holds the Lua script that is used to revise replicas in the desired specification.
	// The script should implement a function as follows:
	//     luaScript: >
	//         function ReviseReplica(desiredObj, desiredReplica)
	//             desiredObj.spec.replicas = desiredReplica
	//             return desiredObj
	//         end
	//
	// LuaScript only holds the function body part, take the `desiredObj` and `desiredReplica` as the function
	// parameters or global variables, and finally returns a revised specification.
	// +required
	LuaScript string `json:"luaScript"`
}

// StatusReflection holds the scripts for getting the status.
type StatusReflection struct {
	// LuaScript holds the Lua script that is used to get the status from the observed specification.
	// The script should implement a function as follows:
	//     luaScript: >
	//         function ReflectStatus(observedObj)
	//             status = {}
	//             status.readyReplicas = observedObj.status.observedObj
	//             return status
	//         end
	//
	// LuaScript only holds the function body part, take the `observedObj` as the function
	// parameters or global variables, and finally returns the status.
	// +required
	LuaScript string `json:"luaScript"`
}

// StatusAggregation holds the scripts for aggregating several decentralized statuses.
type StatusAggregation struct {
	// LuaScript holds the Lua script that is used to aggregate decentralized statuses
	// to the desired specification.
	// The script should implement a function as follows:
	//     luaScript: >
	//         function AggregateStatus(desiredObj, statusItems)
	//             for i = 1, #items do
	//                 desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + items[i].readyReplicas
	//             end
	//             return desiredObj
	//         end
	//
	// LuaScript only holds the function body part, take the `desiredObj` and `statusItems` as the function
	// parameters or global variables, and finally returns the desiredObj.
	// +required
	LuaScript string `json:"luaScript"`
}

// HealthInterpretation holds the rules for interpreting the health state of a specific resource.
type HealthInterpretation struct {
	// LuaScript holds the Lua script that is used to assess the health state of
	// a specific resource.
	// The script should implement a function as follows:
	//     luaScript: >
	//         function InterpretHealth(observedObj)
	//             if observedObj.status.readyReplicas == observedObj.spec.replicas then
	//                 return true
	//             end
	//         end
	//
	// LuaScript only holds the function body part, take the `observedObj` as the function
	// parameters or global variables, and finally returns the boolean health state.
	// +required
	LuaScript string `json:"luaScript"`
}

// DependencyInterpretation holds the rules for interpreting the dependent resources
// of a specific resources.
type DependencyInterpretation struct {
	// LuaScript holds the Lua script that is used to interpret the dependencies of
	// a specific resource.
	// The script should implement a function as follows:
	//     luaScript: >
	//         function GetDependencies(desiredObj)
	//             dependencies = {}
	//             if desiredObj.spec.serviceAccountName ~= "" and desiredObj.spec.serviceAccountName ~= "default" then
	//                 dependency = {}
	//                 dependency.apiVersion = "v1"
	//                 dependency.kind = "ServiceAccount"
	//                 dependency.name = desiredObj.spec.serviceAccountName
	//                 dependency.namespace = desiredObj.namespace
	//                 dependencies[0] = {}
	//                 dependencies[0] = dependency
	//             end
	//             return dependencies
	//         end
	//
	// LuaScript only holds the function body part, take the `desiredObj` as the function
	// parameters or global variables, and finally returns the dependent resources.
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
