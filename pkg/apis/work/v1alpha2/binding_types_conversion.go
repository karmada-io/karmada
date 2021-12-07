package v1alpha2

import "sigs.k8s.io/controller-runtime/pkg/conversion"

// Check if our ResourceBinding implements necessary interface
var _ conversion.Hub = &ResourceBinding{}

// Check if our ClusterResourceBinding implements necessary interface
var _ conversion.Hub = &ClusterResourceBinding{}

// Hub marks this type as a conversion hub.
func (*ResourceBinding) Hub() {}

// Hub marks this type as a conversion hub.
func (*ClusterResourceBinding) Hub() {}
