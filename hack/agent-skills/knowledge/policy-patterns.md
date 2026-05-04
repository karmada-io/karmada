# Karmada Policy Patterns

Idioms an agent should reach for *first*, before inventing a custom shape.

## 1. Regional spread

**When:** "deploy across all EU clusters", "spread across 3 zones", etc.

Prefer `spreadConstraints` with `spreadByField` over hand-listing cluster
names. It survives cluster churn and self-heals when a cluster is
decommissioned.

```yaml
placement:
  clusterAffinity:
    labelSelector:
      matchLabels:
        location: eu
  spreadConstraints:
    - spreadByField: region
      maxGroups: 3
      minGroups: 2
  replicaScheduling:
    replicaSchedulingType: Divided
    replicaDivisionPreference: Weighted
    weightPreference:
      dynamicWeight: AvailableReplicas
```

## 2. Active / passive failover

**When:** "deploy to primary, fall back to secondary if primary is down".

Use **`clusterAffinities`** (plural). The scheduler walks the list and only
moves to the next group if the previous one cannot satisfy the workload.

```yaml
placement:
  clusterAffinities:
    - affinityName: primary
      clusterNames: [member-prod-east]
    - affinityName: backup
      clusterNames: [member-prod-west]
```

Pair with `failover.application` so in-flight pods drain rather than
disappearing:

```yaml
failover:
  application:
    decisionConditions:
      tolerationSeconds: 120
    purgeMode: Gracefully
    gracePeriodSeconds: 60
```

## 3. Per-cluster image registry override

**When:** China clusters need a mirrored registry, EU clusters use the
upstream registry.

Use `imageOverrider`, **not** a raw plaintext patch. It applies to every
container without you having to enumerate JSON Pointer paths.

```yaml
overrideRules:
  - targetCluster:
      labelSelector:
        matchLabels:
          location: cn
    overriders:
      imageOverrider:
        - component: Registry
          operator: replace
          value: registry.cn-hangzhou.aliyuncs.com/myorg
```

## 4. Replicas: duplicated vs divided

| Intent                                            | Setting                    |
|---------------------------------------------------|----------------------------|
| `replicas: 3` should mean *3 on each cluster*     | `replicaSchedulingType: Duplicated` |
| `replicas: 3` should mean *3 total, split across* | `replicaSchedulingType: Divided`    |

If the user says "I want 3 replicas spread across the EU", that is
**Divided** with `Weighted` division. Saying "I want HA, three pods per
region" is **Duplicated**.

## 5. Propagating dependencies

A `Deployment` that mounts a `ConfigMap` will *not* function in member
clusters unless the ConfigMap is also propagated. Two options:

1. Add the ConfigMap to `resourceSelectors` of the same policy.
2. Set `propagateDeps: true` and let Karmada follow references.

Option 2 is preferred when dependencies are likely to grow.

## 6. Suspending dispatch (maintenance)

```yaml
suspension:
  dispatching: false
  dispatchingOnClusters:
    clusterNames: [member-prod-east]
```

This pauses propagation to a specific cluster without deleting the
policy or the workload. Useful for in-place cluster maintenance.

## 7. Conflict resolution

When the same resource is targeted by multiple policies (or already
exists in the member cluster), set `conflictResolution: Overwrite` only
if you actually own that resource on the target. The default `Abort` is
the safe choice for any non-greenfield rollout.
