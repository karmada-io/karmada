---
title: Support Priority Class Configuration for Karmada Control Plane Components
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2025-01-01

---

# Support Priority Class Configuration for Karmada Control Plane Components

## Summary

This proposal aims to extend the Karmada operator by introducing support for configuring the priority class name for Karmada control plane components.
By enabling users to configure a custom priority class name, this feature ensures critical components are scheduled with appropriate priority, enhancing overall system reliability and stability.

## Motivation

Currently, the priority class name for Karmada components is hardcoded to `system-node-critical` for some components, while others do not specify a priority class at all. This limitation can compromise 
the reliability and stability of the system in environments where scheduling of critical components is essential.

By allowing users to configure the priority class name, this feature ensures:

- Greater control over scheduling of critical Karmada control plane components, enhancing system reliability and stability.
- Alignment with organizational policies for resource prioritization and workload management.
- Flexibility to adapt priority classes for specific operational environments and use cases.

### Goals
- Provide a mechanism for configuring the scheduling priority of all in-cluster Karmada control plane components.
- Ensure the feature integrates seamlessly with existing deployments while maintaining backward compatibility.

### Non-Goals

- Address scheduling priorities for components outside the Karmada control plane.

## Proposal

Introduce a new optional `priorityClassName` field in the `CommonSettings` struct, which is used across all Karmada components.

### API Changes

```go
// CommonSettings describes the common settings of all Karmada Components.
type CommonSettings struct {
	
    // PriorityClassName specifies the priority class name for the component.
    // If not specified, it defaults to "system-node-critical".
    // +kubebuilder:default="system-node-critical"
    // +optional
    PriorityClassName string `json:"priorityClassName,omitempty"`
    
    // Other, existing fields omitted for brevity...
}

```
### User Stories

#### Story 1
As an infrastructure engineer, I need to configure the priority class for Karmada control plane components to ensure critical components are reliably scheduled to ensure system stability and reliability.

#### Story 2
As an infrastructure engineer managing a multi-tenant cluster, I want the ability to override the default  priority class for Karmada control plane components with a custom priority class that aligns with my organization’s policies, ensuring reliable resource allocation and system stability across workloads.

### Risks and Mitigations

1. *Backward Compatibility*: Existing deployments might rely on the current hardcoded `system-node-critical` priority class for some components.

    - *Mitigation*: The `priorityClassName` field defaults to `system-node-critical` when not explicitly specified, preserving the current behavior.

## Design Details

During the reconciliation process, the Karmada operator will:

- Check if `priorityClassName` is specified in the component’s `CommonSettings`.
- If specified:
  - Apply the specified priority class to the component’s Pod spec.
- If not specified:
  - Default to `system-node-critical` to maintain backward compatibility.
