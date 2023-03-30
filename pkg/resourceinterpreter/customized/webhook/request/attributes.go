package request

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// Attributes Attributes contains the attributes that call webhook.
type Attributes struct {
	Operation        configv1alpha1.InterpreterOperation
	Object           *unstructured.Unstructured
	ObservedObj      *unstructured.Unstructured
	ReplicasSet      int32
	AggregatedStatus []workv1alpha2.AggregatedStatusItem
}

// ResponseAttributes contains the attributes that response by the webhook.
type ResponseAttributes struct {
	Successful          bool
	Status              configv1alpha1.RequestStatus
	Replicas            int32
	ReplicaRequirements *workv1alpha2.ReplicaRequirements
	Dependencies        []configv1alpha1.DependentObjectReference
	Patch               []byte
	PatchType           configv1alpha1.PatchType
	RawStatus           runtime.RawExtension
	Healthy             bool
}
