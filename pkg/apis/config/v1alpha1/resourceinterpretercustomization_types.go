package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	// APIVersion represents the API version of the target resource.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of target resources.
	// +required
	Kind string `json:"kind"`

	// Retention describes the desired behavior that karmada should react on
	// the changes made by member cluster components. This avoids system
	// running into a meaningless loop that Karmada resource controller and
	// the member cluster component continually applying opposite values of a field.
	// For example, the "replicas" of Deployment might be changed by the HPA
	// controller on member cluster. In this case, Karmada should retain the "replicas"
	// and not try to change it.
	// +optional
	Retention LocalValueRetention `json:"retention,omitempty"`
}

// LocalValueRetention holds the scripts for retention.
// Now only supports Lua.
type LocalValueRetention struct {
	// RetentionLua holds the Lua script that is used to retain runtime values to the desired specification.
	// The script should implement a function as follows:
	//     retentionLua: >
	//         function Retain(desiredObj, observedObj)
	//             desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo
	//             return desiredObj
	//         end
	//
	// RetentionLua only holds the function body part, take the `desiredObj` and `observedObj` as the function
	// parameters or global variables, and finally returns a retained specification.
	// +required
	RetentionLua string `json:"retentionLua"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceInterpreterCustomizationList contains a list of ResourceInterpreterCustomization.
type ResourceInterpreterCustomizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceInterpreterCustomization `json:"items"`
}
