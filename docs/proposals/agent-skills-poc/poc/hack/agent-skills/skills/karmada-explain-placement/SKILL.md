---
name: karmada-explain-placement
description: Given a PropagationPolicy and a fleet view, predict which clusters will receive replicas and explain why, including divided-replica math. Use when the user asks "where will this end up?" or "why did it land on member1 and not member2?".
metadata:
  type: workflow
  loads:
    - knowledge/00-overview.md
    - knowledge/01-policy-patterns.md
    - knowledge/10-propagation-schema.md
  status: scaffold
---

# karmada-explain-placement

> **Status:** scaffold. The deterministic helper (`scripts/explain_placement.py`) is
> planned for mentorship weeks 6–7. This SKILL.md documents the intended workflow so
> reviewers can see the shape; it is functional as an LLM-only skill today by quoting
> the placement section of the user's YAML and walking the scheduler's logic by hand.

## Planned workflow

1. Receive a PropagationPolicy YAML and a `karmadactl get clusters -o yaml` dump.
2. For each cluster, evaluate `placement.clusterAffinity` / `clusterAffinities`:
   - name-list mode: literal membership check
   - label-selector mode: evaluate labels on the Cluster object
   - ordered-groups mode: walk groups in order; first group with ≥1 match wins
3. If `replicaScheduling.replicaSchedulingType = Duplicated`, every selected cluster
   gets the full `spec.replicas` count.
4. If `Divided + Weighted + staticWeightList`, compute integer replica counts
   proportional to weight; round down with remainder to highest-weight cluster.
5. If `Divided + Weighted + dynamicWeight: AvailableReplicas`, the helper cannot know
   live capacity — return a symbolic answer ("scheduler will divide proportional to
   each cluster's AvailableReplicas at decision time").

## Output shape

```
Targets (3 clusters):
- member1  (matched: environment=production)  → 6 replicas (weight 3/5 of 10)
- member2  (matched: environment=production)  → 4 replicas (weight 2/5 of 10)
- member3  (skipped: environment=staging, no match)

Why: clusterAffinity uses a label selector matchLabels={environment: production},
so member3 is filtered out. Weighted division with weights 3:2 over 10 replicas
gives 6:4. See knowledge/01-policy-patterns.md pattern P4.
```

## Why this skill needs a helper, not just an LLM

The static-weight division and the ordered-groups short-circuit both have edge cases
where a smart-but-fuzzy answer is exactly the wrong kind of answer. Once implemented,
the helper makes the math deterministic so the LLM only has to narrate it.
