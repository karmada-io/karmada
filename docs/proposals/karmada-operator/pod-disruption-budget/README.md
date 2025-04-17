---
title: Support PodDisruptionBudget Configuration for Karmada Control Plane Components
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"
creation-date: 2025-04-09
---

# Support PodDisruptionBudget Configuration for Karmada Control Plane Components

## Summary

This proposal aims to extend the Karmada operator by introducing configurable PodDisruptionBudgets (PDBs) for Karmada control plane components. By allowing users to specify the configuration of pod disruption budgets, the operator can create pod disruptions budgets to ensure the availability of control plane components during planned disruptions.
Currently, the Karmada Helm chart already supports creating PDBs for the control plane components. Extending similar functionality to the operator will bring feature parity between installation methods and help users maintain consistent HA policies.

---

## Motivation

1. **Ensuring High Availability**  
   PodDisruptionBudgets help prevent too many replicas of a control plane component from being evicted at once during planned cluster disruption events (e.g., node drains and cluster upgrades). This is especially critical for Karmada’s control plane, where maintaining availability during planned disruptions is essential to maintain continuity of service.

2. **Consistent User Experience**  
   Although the Helm chart for Karmada supports specifying PDBs, this feature is not currently available in the Karmada Operator. Adding PDB support ensures feature parity across installation methods, so users can choose either approach without sacrificing important functionality.

3. **Better Resilience in Production**  
   Production-grade environments with strict SLAs require minimal downtime and reliable upgrades. PDBs, as part of Kubernetes’ core toolset, help achieve graceful rolling updates. Supporting them in the operator means organizations can safely adopt Karmada without risking unexpected downtime for critical control plane components.

4. **Seamless Integration with Cluster Policies**  
   Many Kubernetes operators require PDBs for mission-critical workloads to comply with organizational standards. Extending PDB support to Karmada’s control plane via the operator aligns with these best practices, facilitating compliance and governance.

5. **Avoids Manual Workarounds**  
   Users who need PDBs with the operator-based installation currently must create them manually, adding extra steps and the potential for misconfiguration. First-class PDB support streamlines this process and ensures everything is set up correctly from the start.

---

## Goals

- Provide users an optional way to define the settings of pod disruption budgets for Karmada control plane components.
- Automatically generate the corresponding PDB resources for each control plane component to ensure availability during planned disruptions.
- Maintain backward compatibility for existing Karmada operator deployments where PDBs are not required.

---

## Non-Goals

- Expose the full breadth of PDB configuration options (e.g., `selector` or `unhealthyPodEvictionPolicy`).
- Provide advanced policies for unhealthy pod eviction. At this stage, we focus only on the most common (and stable) PDB fields.

---

## Proposal

---

### API Changes

Add an optional field named `podDisruptionBudgetConfig` to the `CommonSettings` struct. This struct is used across all control plane components . The user can either provide `minAvailable` or `maxUnavailable` to define the PDB’s disruption budget.

```go
// CommonSettings describes the common settings of all Karmada Components.
type CommonSettings struct {
    // Existing fields omitted for brevity...

    // PodDisruptionBudgetConfig specifies the PodDisruptionBudget configuration
    // for this component’s pods. If not set, no PDB will be created.
    // +optional
    PodDisruptionBudgetConfig *PodDisruptionBudgetConfig `json:"podDisruptionBudgetConfig,omitempty"`
}

// PodDisruptionBudgetConfig defines a subset of PodDisruptionBudgetSpec fields
// that users can configure for their control plane components.
type PodDisruptionBudgetConfig struct {
    // minAvailable specifies the minimum number or percentage of pods 
    // that must remain available after evictions.
    // Mutually exclusive with maxUnavailable.
    // +optional
    MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

    // maxUnavailable specifies the maximum number or percentage of pods
    // that can be unavailable after evictions.
    // Mutually exclusive with minAvailable.
    // +optional
    MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}
```
---

## User Stories

1. **High Availability During Planned Disruptions**  
   As a cloud infrastructure engineer managing multiple clusters with Karmada control planes, I need to ensure that voluntary disruption, such as node drains or rolling upgrades, do not evict too many replicas at once. By defining PodDisruptionBudgets through the operator, I can maintain control plane availability and keep workloads across member clusters in sync, even during maintenance windows.

2. **Consistent Experience Across Installation Methods**  
   As a cloud infrastructure engineer responsible for deploying Karmada in various ways, I want the same fine-grained control over PodDisruptionBudgets whether I'm using Helm or the Karmada Operator. This consistency simplifies my workflow, prevents confusion, and lets me standardize on a single set of eviction policies.

3. **Improved Resilience for Production Environments**  
   As a cloud infrastructure engineer working under strict Service Level Agreements (SLAs), I cannot afford unplanned downtime in the Karmada control plane. PDBs are vital for graceful rolling updates and node maintenance, and having them supported natively by the operator ensures the control plane remains stable and resilient in production scenarios.

4. **Alignment with Organizational Policies**  
   As a cloud infrastructure engineer in an organization with strict compliance standards, I need to enforce PodDisruptionBudgets for critical workloads. Integrating PDB support directly into the operator makes it straightforward to adhere to these policies, eliminating the risk of manual misconfiguration and ensuring compliance from the start.

5. **Avoiding Manual Workarounds**  
   As a cloud infrastructure engineer provisioning Karmada with the operator, I currently have to create PDBs manually afterward, which is easy to overlook or configure incorrectly. By having first-class PDB support, I can rely on the operator to automatically set up disruption budgets, guaranteeing my control plane meets high-availability requirements from day one.

---

## Risks and Mitigations

### Misconfiguration of PDB
- If a user sets `minAvailable` higher than the number of replicas, the pods could become unschedulable during updates or evictions.  
  **Mitigation**: The operator could report an error during validation of the Karmada resource if `replicas < minAvailable`.

### Backward Compatibility
- Existing installations don’t create or expect PDBs.  
  **Mitigation**: The new fields are optional; if not set, no PDB is created, preserving current behavior.

---

## Design Details

### Workflow

#### Reconcile Logic
- For each control plane component, the operator checks if `podDisruptionBudgetConfig` is set.
    - If set, the operator constructs or updates a `PodDisruptionBudget` object using the provided values.
    - If `podDisruptionBudgetConfig` is **nil**, no PDB is created for that component.

### Default Values
- By default, `podDisruptionBudgetConfig` is `nil`, which results in **no** PDB for a component.
- If the user chooses to enable a PDB, they **must** supply a valid `MinAvailable` **or** `MaxUnavailable` (but not both).

---

## Alternatives

### Manual PDB Creation
Users can continue to create PDBs by hand, but this is error-prone and lacks the operator-driven, declarative approach many users prefer.

### Expand API for Full PDB Control
The operator could expose the entire `PodDisruptionBudgetSpec` structure, but this adds complexity and can introduce problem stemming from issues such as misconfigured label selectors.

---

## Implementation Plan

1. **Add `podDisruptionBudgetConfig` to `CommonSettings`** in the Karmada operator CRDs.
2. **Update the controller reconcile logic** to:
    - Inspect `podDisruptionBudgetConfig` for each control plane component.
    - Create or update a PDB object as needed.
3. **Documentation**
    - Add usage examples in the Karmada operator docs showing how to specify `podDisruptionBudgetConfig` and explaining best practices for `minAvailable` or `maxUnavailable`.

---

## Conclusion
By adding **PodDisruptionBudget (PDB) configuration** to the Karmada operator’s CRD, we provide users with the ability to specify policies to ensure the availability of control plane components during planned disruptions. This improvement will streamline ops workflows, reduce manual steps, and increase reliability of Karmada control planes in production environments.
