---
title: "KEP: Federated Stateful Rollout (Coordinated Blue-Green Migration)"
authors:
  - "@liwang0513"
reviewers:
  - "@RainbowMango"
  - "@XiShanYongYe-Chang"
  - "@zhzhuang-zju"
approvers:
  - "@RainbowMango"
creation-date: 2026-04-04
---

# Federated Stateful Rollout: Coordinated Blue-Green Migration for Flink

## Summary
The **Federated Stateful Rollout** feature introduces a proactive orchestration mechanism for stateful workloads across multiple clusters. While the existing `StatefulFailover` handles unplanned outages (reactive), this feature manages planned operations such as regional rebalancing, cluster maintenance, and safe image upgrades. By coordinating a "Suspend-Capture-Resume" lifecycle, it ensures **Zero Data Loss** and eliminates **Reprocessing Lag** by utilizing synchronous Savepoints.

## Motivation
Standard multi-cluster failover in Karmada currently faces three technical gaps for streaming applications:
* **Reprocessing Lag:** Recovery from stale periodic checkpoints forces jobs to "catch up" on data backlogs, causing downstream latency.
* **Topology Friction:** Image or DAG updates often break compatibility with old checkpoints; coordinated Savepoints are required for safe upgrades.
* **Safety Gap:** There is no "Validation Gate." In standard failover, the source instance is often deleted before the target is confirmed healthy.

### Goals
* **Coordinated Handoff:** Ensure an atomic "baton pass" of state between clusters.
* **Zero Reprocessing:** Use synchronous Savepoints to start the target exactly where the source stopped.
* **Validation Gate:** Keep the source cluster as a "hot standby" until the target is `RUNNING`.
* **Transparent Orchestration:** Automate the manipulation of `ResourceBindings` and `Overrides` within ClusterSets.

### Non-Goals
* Replacing reactive `StatefulFailover`.
* Managing underlying storage (S3/GCS) bucket permissions.

## Proposal: StatefulMigrationController
We propose a new controller in `karmada-controller-manager` that orchestrates the migration state machine.

### Transition State Machine
| Phase | Action | Visibility |
| :--- | :--- | :--- |
| **1. Trigger** | User taints a cluster or adds a migration annotation. | User-Visible |
| **2. Discovery** | Controller identifies active cluster via `ResourceBinding` status. | Transparent |
| **3. Expansion** | Controller patches `ResourceBinding` (replicas: 2) and adds a Finalizer. | Transparent |
| **4. Hold** | Controller applies `ClusterOverridePolicy` to Target (state: `suspended`). | Transparent |
| **5. Capture** | Controller patches Source to `suspended`, triggers Savepoint. | Transparent |
| **6. Handoff** | Controller injects Savepoint URL into Target Override and flips to `running`. | Transparent |
| **7. Cleanup** | Controller removes Source from `ResourceBinding` and deletes Overrides. | Transparent |

## Design Details

### The "Hold" Pattern via ClusterOverridePolicy
To ensure the target cluster does not start prematurely, the controller utilizes a `ClusterOverridePolicy` to "hold" the deployment in a suspended state while the `ResourceBinding` is expanded.

**Example Hold Override:**
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterOverridePolicy
metadata:
  name: flink-migration-hold-pw
spec:
  resourceSelectors:
    - apiVersion: flink.apache.org/v1beta1
      kind: FlinkDeployment
      name: hbase-demo
      namespace: s-spaasapi
  targetCluster:
    clusterNames: ["spaas-kaas-pw-dev02"]
  overriders:
    plaintext:
      - path: "/spec/job/state"
        operator: replace
        value: "suspended"
```
### ResourceBinding Manipulation
In environments with maxGroups: 1, the controller must manually expand the ResourceBinding to allow coexistence during the handoff. A Migration Finalizer is added to prevent the Scheduler from reverting the expansion during the transition window.

```yaml
# Internal ResourceBinding Patch
spec:
  clusters:
    - name: spaas-kaas-tt-dev02 (Source)
      replicas: 1
    - name: spaas-kaas-pw-dev02 (Target)
      replicas: 1
  replicas: 2
```

## User Stories
### Story 1: Planned Cluster Maintenance (0 RPO)
An SRE taints cluster `tt` for a Kubernetes upgrade. The controller detects the intent, captures a synchronous Savepoint in `tt`, and hands it to cluster `pw`. The job resumes in `pw` with zero backlog, maintaining real-time processing.

### Story 2: Atomic Image Upgrade
A developer updates the `FlinkDeployment` image. The controller orchestrates a Blue-Green move. If the new image fails to initialize in the target cluster, the controller aborts and resumes the original job in the source cluster, providing an automated safety net.

## Risks and Mitigations
- Risk: Split-Brain. Multiple clusters writing to the same sink.

    - Mitigation: Strict "Suspend-before-Resume" sequence confirmed via `ResourceInterpreter` status aggregation.

## Alternatives Considered
- Manual Scripting: Rejected as error-prone and unsafe for Exactly-Once requirements.

- New Federated CRD: Rejected to avoid API sprawl. Using standard `FlinkDeployment` + `Karmada` Overrides is more sustainable.

