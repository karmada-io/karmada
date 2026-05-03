---
name: karmada-explain-placement
description: Explain why Karmada is propagating (or not propagating) a workload to a specific set of member clusters by reading the ResourceBinding scheduling result and tracing it back to the PropagationPolicy fields that drove the decision.
when_to_load: |
  Trigger when the user asks: "why is my workload on cluster X?", "why ISN'T
  it on cluster Y?", "how did the scheduler pick these clusters?", "what
  rules selected these clusters?", or any "explain the placement" phrasing.
  Always loads `karmada-knowledge` first.
---

# karmada-explain-placement

An agent answering *"why this cluster?"* must ground every claim in
observable state — the `ResourceBinding`, the `PropagationPolicy`, and
cluster labels — not in plausible-sounding inferences.

## Workflow

### Step 1 — read the actual decision

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> get rb \
  <workload>-<kind> -o yaml
```

The fields that matter:

- `spec.clusters[]` — the cluster list the scheduler chose.
- `status.conditions[?(@.type=="Scheduled")].message` — the scheduler's
  human-readable reason.
- `status.schedulerObservedAffinityName` — when `clusterAffinities`
  (plural) is in use, which group is currently active.

### Step 2 — read the policy that produced the decision

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> get pp <policy> -o yaml
```

Map each `spec.placement.*` field to the decision:

| Policy field                  | What it controls                          |
|-------------------------------|-------------------------------------------|
| `clusterAffinity.labelSelector` | the candidate set                       |
| `clusterAffinity.clusterNames`  | hard-coded candidate set                |
| `clusterAffinities[*]`         | ordered failover groups (plural form)    |
| `clusterTolerations`           | which tainted clusters are eligible      |
| `spreadConstraints`            | how candidates are grouped before pick   |
| `replicaScheduling`            | how replicas are split across the picked clusters |

### Step 3 — verify candidate cluster labels

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG get clusters --show-labels
kubectl --kubeconfig $KARMADA_KUBECONFIG describe cluster <name>
```

If a user expected cluster `member-de` and it was excluded, the cause is
almost always one of:

1. Label mismatch (selector wants `region=eu`, cluster has no `region` label).
2. Taint without matching toleration (cluster has `NoSchedule` taint).
3. `Ready=False` condition on the cluster.
4. `cluster.spec.id` excluded by `clusterAffinity.exclude`.

### Step 4 — explain in this exact structure

Always answer in three parts:

1. **Decision:** "Workload runs on `member-X`, `member-Y`."
2. **Driver:** "Because policy `<name>` has
   `clusterAffinity.labelSelector: {region: eu}` and these are the only
   clusters with that label."
3. **Counterfactual:** "If you wanted `member-cn` included, you would
   need to either label it `region: eu` (wrong) or change the selector
   to also match its `location: cn` label (right)."

The counterfactual is what makes the explanation actionable.

## Replica explanation

If the user asks *"why does cluster A have 5 pods and cluster B have 1?"*
inspect:

- `placement.replicaScheduling.replicaSchedulingType`
  - `Duplicated` → every cluster gets full `spec.replicas`. If they
    differ, an OverridePolicy is patching `/spec/replicas` per cluster.
  - `Divided` with `Weighted` and `weightPreference.staticWeightList`
    → sum of weights determines the split.
  - `Divided` with `dynamicWeight: AvailableReplicas` → driven by live
    cluster capacity reported in `Cluster.status`.

## What the agent must NOT do

- **Don't guess label semantics.** If the user's clusters don't have a
  `region` label, the scheduler did not match on `region`. Period.
- **Don't conflate `clusterAffinity` (singular) with
  `clusterAffinities` (plural).** They produce different status fields.
