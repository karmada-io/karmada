---
title: Support to Specify Extra Volumes and Volume Mounts for Karmada Control Plane Components
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2024-07-29

---

# Support to Specify Extra Volumes and Volume Mounts for Karmada Control Plane Components


## Summary

This proposal aims to introduce the ability to specify extra volumes and volume mounts for Karmada control plane components when creating a Karmada instance managed by the Karmada operator. 
This enhancement will extend the current configuration capabilities, allowing users to meet additional requirements such as having the ability to configure encryption at rest or to configure
custom authentication webhooks.

## Motivation
Currently, the Karmada operator allows specifying extra arguments for the Karmada control plane components, but lacks support for specifying extra volumes and volume mounts for those components. 
This limitation hinders certain use cases that require additional configurations for security and customization. For instance, as of today, it's not possible to configure a custom authentication 
webhook nor encryption at rest. 

### Goals
- Enable users to specify extra volumes and volume mounts for Karmada control plane components.


### Non-Goals
- Introducing changes to the core functionality of the Karmada control plane components.
- Overhauling the existing configuration of Karmada control plane components beyond the scope of adding extra volumes and volume mounts.


## Proposal
Introduce the new optional fields `ExtraVolumes` and `ExtraVolumeMounts` within Karmada control plane component configurations.

## Design Details

Introduce the new optional fields `ExtraVolumes` and `ExtraVolumeMounts` within Karmada control plane component configurations.
Let's take the API server component as an example. With these new fields, the configuration would look as follows:
```go
// KarmadaAPIServer holds settings to kube-apiserver component of the kubernetes.
// Karmada uses it as its own apiserver in order to provide Kubernetes-native APIs.
type KarmadaAPIServer struct {
	// Other existing fields

	// ExtraVolumes specifies a list of extra volumes for the API server's pod
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts specifies a list of extra volume mounts to be mounted into the API server's container
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}
```
