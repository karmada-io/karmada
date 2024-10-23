---
title: Add API Server Service Information to `KarmadaStatus`
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2024-09-24

---

# Add API Server Service Information to `KarmadaStatus`

## Summary

This proposal aims to enhance `KarmadaStatus` by adding a new field that contains the name and port of the API server service for a Karmada control plane. This change will simplify the process of referencing an API server service,
eliminating the need to infer the service name and exposed client port. This is useful for higher-level operators that may need to perform additional tasks like creating an ingress resource to configure external traffic to the service once a Karmada instance has been provisioned.

## Motivation

When managing a Karmada instance, referencing its API server service (e.g., for creating an ingress resource) currently requires inferring the service name and exposed port from conventions. Relying on this method is brittle since it depends on internal implementation details that may change.

By including the API server service information directly in the `KarmadaStatus` field, operators can directly reference the service name and exposed port, improving reliability and simplifying cluster management.

### Goals

- Add a new field to `KarmadaStatus` to store the API server service information.
- Ensure the API server service information is accessible to operators or higher-level systems.

### Non-Goals

- Modify the process of how the Karmada API server service is created.
- Affect backward compatibility of the existing `KarmadaStatus`.

## Proposal

The proposal is to introduce a new field, `APIServerServiceStatus`, in `KarmadaStatus` to capture the name and port of the Karmada API server service. This would provide a more reliable method for referencing the service when creating additional resources, like `Ingress` objects.


### User Stories

#### Story 1
As an operator, I want to provision an Ingress for the Karmada API server without needing to guess or infer the service name and port.

#### Story 2
As an administrator, I want to ensure the control plane's API server service is reliably discoverable by systems provisioning resources on top of Karmada.

### Risks and Mitigations

This change introduces minimal risk as it is backward-compatible. The new field will be optional, and systems that do not need it can safely ignore it. 
Testing should focus on ensuring that the Karmada operator correctly populates the new field in `KarmadaStatus`.

## Design Details
With the proposed changes, `KarmadaStatus` will have the following structure:

```go
// KarmadaStatus defines the most recently observed status of the Karmada.
type KarmadaStatus struct {
	// Other existing fields...

	// APIServerServiceStatus contains the current status of the Karmada API server service.
	// +optional
	APIServerServiceStatus APIServerServiceStatus `json:"apiServerServiceStatus,omitempty"`
}

// APIServerServiceStatus contains the current status of the API server service.
type APIServerServiceStatus struct {
    // Name represents the name of the service.
    Name string `json:"name"`
    
    // ServiceType represents the service type
    ServiceType corev1.ServiceType `json:"serviceType"`
    
    // Port represents the port exposed by the service.
    Port int32 `json:"port"`
    
    // NodePort represents the node port on which the service is exposed
    // when service is of type NodePort or LoadBalancer
    // +optional
    NodePort int32 `json:"nodePort"`
    
    // LoadBalancerStatus represents the status of a load-balancer when
    // the service is of type LoadBalancer
    // +optional
    LoadBalancerStatus corev1.LoadBalancerStatus `json:"loadBalancerStatus"`
}
```
The Karmada operator will need to be updated to populate the `APIServerServiceStatus` field during its reconciliation process.