---
title: Support Suspension of Reconciliation for Karmada Control Planes
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"
creation-date: 2025-05-08
---

# Support Suspension of Reconciliation for Karmada Control Planes

## Summary

This proposal adds support to pause/suspend the reconciliation of a Karmada resource by the operator. When paused/suspended, the operator should refrain from enqueuing reconciliation requests for changes to the primary `Karmada` resource, or any of its related objects.


## Motivation

1. **Emergency Troubleshooting**  
   Platform engineers can hot‑patch components without having the Karmada operator overwrite their critical emergency patches.

2. **Coordinated Maintenance**  
   During multi‑step upgrades, teams can freeze reconciliation of control planes, validate each phase, then proceed.

3. **Blast-Radius Control**  
   Pausing the reconcile loop of a resource during incidents prevents automation that can worsen outages.

4. **Ecosystem Consistency**  
   Other CNCF project operators expose a pause/suspend flag; adopting the same pattern will feel familiar to users and tooling.

## Goals

- Provide an opt-in mechanism to pause reconciliation for an individual `Karmada` resource.
- Resume reconciliation deterministically when the pause is removed.
- Maintain full backward compatibility— `Karmada` objects that do not use the feature will behave exactly as they do today.

## Non-Goals

- Pausing at a per-component granularity (e.g., only the karmada scheduler).


## Proposal

### API Changes

Add the optional `paused` **boolean** field to the `Karmada` spec:

```go
// KarmadaSpec defines the desired state of Karmada.
type KarmadaSpec struct {
    // ...existing fields...
    
    // Paused indicates that the operator should suspend reconciliation
    // for this Karmada control plane and all its managed resources.
    // +optional
    Paused bool `json:"paused,omitempty"`
}
```

## User Stories

1. **Emergency Troubleshooting**  
   As a platform engineer, I want the ability to hot‑patch control plane components without having the Karmada operator overwrite my critical emergency patches.

2. **Blast‑radius Control**  
   As a platform engineer, I want the ability to pause the reconcile loop of a resource during incidents to prevents automation that can worsen outages.


## Risks and Mitigations

### Backward Compatibility
- Existing installations are not aware of this new field.  
  **Mitigation**: The new field is completely optional; if not explicitly set to `true`, all existing objects will continue to be reconciled as they do today.


## Implementation Plan

1. Add `paused` boolean field to the `KarmadaSpec` struct.
2. Update the controller reconcile logic to:
    - Inspect the `paused` field for each `Karmada` object to be reconciled. If set to `true`, the controller will refrain from enqueueing reconciliation requests
   for the object. Otherwise, reconciliation will continue to behave as is today.