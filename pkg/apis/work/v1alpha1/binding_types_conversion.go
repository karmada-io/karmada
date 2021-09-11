package v1alpha1

import (
	"unsafe"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// Check if our ResourceBinding implements necessary interface
var _ conversion.Convertible = &ResourceBinding{}

// Check if our ClusterResourceBinding implements necessary interface
var _ conversion.Convertible = &ClusterResourceBinding{}

// ConvertTo converts this ResourceBinding to the Hub version.
func (rb *ResourceBinding) ConvertTo(dstRaw conversion.Hub) error {
	hub := dstRaw.(*workv1alpha2.ResourceBinding)
	hub.ObjectMeta = rb.ObjectMeta

	ConvertBindingSpecToHub(&rb.Spec, &hub.Spec)
	ConvertBindingStatusToHub(&rb.Status, &hub.Status)

	return nil
}

// ConvertFrom converts ResourceBinding from the Hub version to this version.
func (rb *ResourceBinding) ConvertFrom(srcRaw conversion.Hub) error {
	hub := srcRaw.(*workv1alpha2.ResourceBinding)
	rb.ObjectMeta = hub.ObjectMeta

	ConvertBindingSpecFromHub(&hub.Spec, &rb.Spec)
	ConvertBindingStatusFromHub(&hub.Status, &rb.Status)

	return nil
}

// ConvertTo converts this ClusterResourceBinding to the Hub version.
func (rb *ClusterResourceBinding) ConvertTo(dstRaw conversion.Hub) error {
	hub := dstRaw.(*workv1alpha2.ClusterResourceBinding)
	hub.ObjectMeta = rb.ObjectMeta

	ConvertBindingSpecToHub(&rb.Spec, &hub.Spec)
	ConvertBindingStatusToHub(&rb.Status, &hub.Status)

	return nil
}

// ConvertFrom converts ClusterResourceBinding from the Hub version to this version.
func (rb *ClusterResourceBinding) ConvertFrom(srcRaw conversion.Hub) error {
	hub := srcRaw.(*workv1alpha2.ClusterResourceBinding)
	rb.ObjectMeta = hub.ObjectMeta

	ConvertBindingSpecFromHub(&hub.Spec, &rb.Spec)
	ConvertBindingStatusFromHub(&hub.Status, &rb.Status)

	return nil
}

// ConvertBindingSpecToHub converts ResourceBindingSpec to the Hub version.
// This function intends to be shared by ResourceBinding and ClusterResourceBinding.
func ConvertBindingSpecToHub(src *ResourceBindingSpec, dst *workv1alpha2.ResourceBindingSpec) {
	// TODO(RainbowMango): currently v1alpha1 has introduced break changes at PR#637 and PR#657, we can revert it back before
	// next release and updates the hub version to v1alpha2.
	dst.Resource.APIVersion = src.Resource.APIVersion
	dst.Resource.Kind = src.Resource.Kind
	dst.Resource.Namespace = src.Resource.Namespace
	dst.Resource.Name = src.Resource.Name
	dst.Resource.ResourceVersion = src.Resource.ResourceVersion
	dst.ReplicaRequirements = (*workv1alpha2.ReplicaRequirements)(unsafe.Pointer(src.ReplicaRequirements))
	dst.Replicas = src.Replicas

	for i := range src.Clusters {
		dst.Clusters = append(dst.Clusters, workv1alpha2.TargetCluster(src.Clusters[i]))
	}
}

// ConvertBindingStatusToHub converts ResourceBindingStatus to the Hub version.
// This function intends to be shared by ResourceBinding and ClusterResourceBinding.
func ConvertBindingStatusToHub(src *ResourceBindingStatus, dst *workv1alpha2.ResourceBindingStatus) {
	dst.Conditions = src.Conditions
	for i := range src.AggregatedStatus {
		dst.AggregatedStatus = append(dst.AggregatedStatus, workv1alpha2.AggregatedStatusItem(src.AggregatedStatus[i]))
	}
}

// ConvertBindingSpecFromHub converts ResourceBindingSpec from the Hub version.
// This function intends to be shared by ResourceBinding and ClusterResourceBinding.
func ConvertBindingSpecFromHub(src *workv1alpha2.ResourceBindingSpec, dst *ResourceBindingSpec) {
	// TODO(RainbowMango): currently v1alpha1 has introduced break changes at PR#637 and PR#657, we can revert it back before
	// next release and update the hub version to v1alpha2.
	dst.Resource.APIVersion = src.Resource.APIVersion
	dst.Resource.Kind = src.Resource.Kind
	dst.Resource.Namespace = src.Resource.Namespace
	dst.Resource.Name = src.Resource.Name
	dst.Resource.ResourceVersion = src.Resource.ResourceVersion
	dst.ReplicaRequirements = (*ReplicaRequirements)(unsafe.Pointer(src.ReplicaRequirements))
	dst.Replicas = src.Replicas

	for i := range src.Clusters {
		dst.Clusters = append(dst.Clusters, TargetCluster(src.Clusters[i]))
	}
}

// ConvertBindingStatusFromHub converts ResourceBindingStatus from the Hub version.
// This function intends to be shared by ResourceBinding and ClusterResourceBinding.
func ConvertBindingStatusFromHub(src *workv1alpha2.ResourceBindingStatus, dst *ResourceBindingStatus) {
	dst.Conditions = src.Conditions
	for i := range src.AggregatedStatus {
		dst.AggregatedStatus = append(dst.AggregatedStatus, AggregatedStatusItem(src.AggregatedStatus[i]))
	}
}
