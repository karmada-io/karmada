---
title: Add Support for Specifying Node Selectors for Components Provisioned by the Operator
authors:
  - "@jabellard"
reviewers:
  - "@RainbowMango"
approvers:
  - "@RainbowMango"

creation-date: 2026-04-26

---

# Add Support for Specifying Node Selectors for Components Provisioned by the Operator

## Summary

This proposal adds a `NodeSelector` field to the `CommonSettings` struct in the Karmada operator API. This allows users to constrain Karmada control plane component pods to nodes with specific labels using the simplest available Kubernetes node selection mechanism.

---

## Motivation

Not all Karmada control plane components have the same resource profile. `karmada-search` is significantly more resource-hungry than other components like `karmada-webhook` or `karmada-scheduler`. In clusters with small nodes, `karmada-search` may not be schedulable at all — or may cause resource pressure that destabilizes other components on the same node.

A common solution is to add a node pool with larger VMs, label them (e.g., `nodepool: large-vm`), and pin `karmada-search` to that pool. The large-VM nodes are shared — other pods can run there too — so taints are not appropriate. What's needed is a simple way to say "this component must run on nodes with this label."

Today, the only way to achieve node selection through `CommonSettings` is via the `Affinity` field, which requires constructing a `nodeAffinity` with `nodeSelectorTerms` and `matchExpressions`. For this common case, that's unnecessarily verbose:

**With nodeSelector (proposed):**
```yaml
nodeSelector:
  nodepool: large-vm
```

**With nodeAffinity (current):**
```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: nodepool
              operator: In
              values:
                - large-vm
```

`nodeSelector` and `nodeAffinity` are complementary — Kubernetes evaluates both when present. `nodeSelector` handles simple cases concisely, while `nodeAffinity` handles complex logic (`In`, `NotIn`, `Gt`, `Lt`, soft preferences).

---

## Goals

- Allow users to configure `nodeSelector` on any Karmada control plane component via the `Karmada` CR.
- Follow the existing patcher pattern.

---

## Non-Goals

- Merging user-provided `nodeSelector` with any component defaults. If a user specifies `nodeSelector`, their value is used as-is.

---

## Proposal

---

### User Stories

1. **Pinning a resource-hungry component to a large-VM node pool**
   As a platform engineer, my cluster has small nodes and `karmada-search` is too resource-hungry to run on them. I've added a node pool with larger VMs labeled `nodepool: large-vm`. I want to pin `karmada-search` to that pool so it's guaranteed to land on nodes with enough capacity, while leaving other components on the existing smaller nodes. The large-VM nodes are shared — other pods can run there too — so taints are not appropriate. `nodeSelector` is the right fit: it constrains `karmada-search` to the large-VM pool without affecting any other workloads.

   ```yaml
   apiVersion: operator.karmada.io/v1alpha1
   kind: Karmada
   metadata:
     name: karmada
     namespace: karmada-system
   spec:
     components:
       karmadaSearch:
         replicas: 2
         nodeSelector:
           nodepool: large-vm
   ```
   
---

## Design Details

### API Change

Add the following field to `CommonSettings`:

```go
// NodeSelector is a map of key-value pairs that must match a node's labels for the
// pod to be scheduled on that node. This is the simplest form of node selection.
// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector
// +optional
NodeSelector map[string]string `json:"nodeSelector,omitempty"`
```

### Patcher Change

Add a `WithNodeSelector` builder method to the patcher and apply it in both `ForDeployment` and `ForStatefulSet`:

```go
func (p *Patcher) WithNodeSelector(nodeSelector map[string]string) *Patcher {
    p.nodeSelector = nodeSelector
    return p
}
```

```go
// In ForDeployment and ForStatefulSet:
if len(p.nodeSelector) > 0 {
    podSpec.NodeSelector = p.nodeSelector
}
```

---

### Test Plan

- **Unit tests:** Extend patcher unit tests to verify `NodeSelector` is correctly applied to both Deployment and StatefulSet pod specs, and that an empty map results in no change.
- **Integration tests:** Add test cases for at least one Deployment-based component and the etcd StatefulSet to verify end-to-end wiring.
