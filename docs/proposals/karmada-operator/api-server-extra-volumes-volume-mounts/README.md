---
title: Support to Specify Extra Volumes and Volume Mounts for Karmada API Server Component
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2024-07-29

---

# Support to Specify Extra Volumes and Volume Mounts for Karmada API Server Component


## Summary

This proposal aims to introduce the ability to specify extra volumes and volume mounts for the Karmada API server component when creating a Karmada instance managed by the Karmada operator. 
This enhancement will extend the current configuration capabilities, allowing users to meet additional requirements such as having the ability to configure encryption at rest or to configure
custom authentication webhooks.

## Motivation
Currently, the Karmada operator allows specifying extra arguments for the Karmada API server component, but lacks support for specifying extra volumes and volume mounts for it. 
This limitation hinders certain use cases that require additional configurations for security and customization. For instance, as of today, it's not possible to configure a custom authentication 
webhook nor encryption at rest. 

### Goals
- Enable users to specify extra volumes and volume mounts for Karmada API server component to unlock the following use cases:
    - Ability to configure a custom authentication webhook
    - Ability to configure encryption at rest

It's important to note that although this support will unlock the aforementioned use cases, given that there is a lot that can be configured for the API server component by having the ability to specify not only extra args, but 
also extra volumes and volume mounts, other potential use cases requiring similar configurations are also unlocked.


### Non-Goals
- Introducing changes to the core functionality of the Karmada API server component.
- Overhauling the existing configuration of the Karmada API server component beyond the scope of adding extra volumes and volume mounts.


## Proposal
Introduce the new optional fields `ExtraVolumes` and `ExtraVolumeMounts` within the Karmada API server component configuration.

## Design Details

Introduce the new optional fields `ExtraVolumes` and `ExtraVolumeMounts` within Karmada API server component configuration.
With these new fields, the configuration would look as follows:
```go
// KarmadaAPIServer holds settings to kube-apiserver component of the kubernetes.
// Karmada uses it as its own apiserver in order to provide Kubernetes-native APIs.
type KarmadaAPIServer struct {
	// Other existing fields

    // ExtraVolumes specifies a list of extra volumes for the API server's pod
    // To fulfil the base functionality required for a functioning control plane, when provisioning a new Karmada instance,
    // the operator will automatically attach volumes for the API server pod needed to configure things such as TLS,
    // SA token issuance/signing and secured connection to etcd, amongst others. However, given the wealth of options for configurability,
    // there are additional features (e.g., encryption at rest and custom AuthN webhook) that can be configured. ExtraVolumes, in conjunction
    // with ExtraArgs and ExtraVolumeMounts can be used to fulfil those use cases.
    // +optional
    ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

    // ExtraVolumeMounts specifies a list of extra volume mounts to be mounted into the API server's container
    // To fulfil the base functionality required for a functioning control plane, when provisioning a new Karmada instance,
    // the operator will automatically mount volumes into the API server container needed to configure things such as TLS,
    // SA token issuance/signing and secured connection to etcd, amongst others. However, given the wealth of options for configurability,
    // there are additional features (e.g., encryption at rest and custom AuthN webhook) that can be configured. ExtraVolumeMounts, in conjunction
    // with ExtraArgs and ExtraVolumes can be used to fulfil those use cases.
    // +optional
    ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}
```
