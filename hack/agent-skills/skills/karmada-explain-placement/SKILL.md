# karmada-explain-placement

## Description
Explain how a Karmada workload will be placed across member clusters given a
PropagationPolicy or ClusterPropagationPolicy, including replica distribution,
spread constraints, and failover behavior.

## When to Load
- User asks "how will this be placed?", "which clusters will get this workload?", "explain the placement".
- User asks about replica distribution or scheduling decisions.
- User provides a PropagationPolicy and wants to understand the scheduling outcome.
- User asks "why did the scheduler choose these clusters?".

## Workflow

1. **Parse the PropagationPolicy.**
   - Extract `resourceSelectors`, `placement`, `replicaScheduling`, `failover`.
   - Determine the workload type and its spec (especially `replicas`).

2. **Resolve target clusters:**
   - If `clusterAffinity.clusterNames` is set → explain these specific clusters are targeted.
   - If `clusterAffinity.labelSelector` is set → explain which clusters match the labels.
   - If `clusterAffinity.fieldSelector` is set → explain which clusters match the field selectors (provider, region, zone).
   - If `clusterAffinity.exclude` is set → note which clusters are excluded.
   - If `clusterAffinities` with multiple groups → explain the group evaluation order.
     - First group evaluated first; if it fails, next group is tried.
     - `overflowAffinities` expand the group progressively when resources are insufficient.
     - Overflow groups contract during scale-down in reverse order.
   - If neither `clusterAffinity` nor `clusterAffinities` is set → any cluster is a candidate.

3. **Explain spread constraints:**
   - `spreadByField`: How the spread field (cluster/region/zone/provider) groups clusters.
   - `spreadByLabel`: How the label groups clusters.
   - `maxGroups` / `minGroups`: Constraints on how many groups can be selected.
   - Default spread by `cluster` if no spread constraint is set.

4. **Explain replica scheduling:**
   - `Duplicated`: Each target cluster gets the full replica count → explain total pods.
   - `Divided` + `Aggregated`: Replicas packed into as few clusters as possible.
   - `Divided` + `Weighted` + `staticWeightList`: Show replica distribution calculation.
   - `Divided` + `Weighted` + `dynamicWeight: AvailableReplicas`: Explain dynamic allocation.
   - Show the math when applicable (e.g., weight 2:1 → 2/3 and 1/3 of replicas).

5. **Explain tolerations:**
   - Any `clusterTolerations`? List them and explain which tainted clusters they allow.

6. **Explain workload affinity/anti-affinity:**
   - Affinity: Workloads with same label value co-located.
   - Anti-affinity: Workloads with same label value separated.
   - Note: First workload in a group is not blocked (allows bootstrapping).

7. **Explain failover behavior:**
   - Application failover: What happens when application fails on a cluster.
     - `decisionConditions.tolerationSeconds`: Wait time before failover.
     - `purgeMode`: Directly, Gracefully (with `gracePeriodSeconds`), or Never.
   - Cluster failover: What happens when a cluster fails.
     - `purgeMode`: Directly or Gracefully.
   - `statePreservation`: Any state data preserved during failover.

8. **Check for edge cases:**
   - If `conflictResolution=Abort` and resource exists on target → placement will fail.
   - If `suspension.dispatching=true` → placement calculated but not executed.
   - If `suspension.dispatchingOnClusters` → some clusters suspended.
   - If `activationPreference=Lazy` → policy changes won't take effect until resource template changes.

9. **Produce structured output:**
   ```markdown
   ## Placement Explanation for <Policy Name>

   ### Target Clusters
   | Cluster | Reason |
   |---------|--------|
   | member1 | Listed in clusterNames |
   | member2 | Listed in clusterNames |

   ### Replica Distribution
   - Total replicas: 6
   - Distribution: member1=4, member2=2 (weighted 2:1)

   ### Failover
   - Application: toleration=300s, purge=Gracefully(600s)
   - Cluster: purge=Gracefully

   ### Notes
   - ...
   ```

## Knowledge References
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/policy-patterns.md`
