# Policy Patterns (reference catalog)

This file lists the placement and override patterns an agent should reach for first.
Each entry has: the intent in plain English, when to use it, the minimal YAML, and the
gotcha that trips up first-time users. Skills load this file when they need to choose
between equally valid YAML shapes.

---

## P1. Name-list placement (the 90% case)

**Intent:** "Put this on member1 and member2."

```yaml
spec:
  placement:
    clusterAffinity:
      clusterNames: [member1, member2]
```

**When:** small fleets, dev/test, anything where the operator knows cluster names by hand.
**Gotcha:** typos here are silent at apply time. The policy is admitted, no binding is
created. `karmadactl get rb` will show nothing. See troubleshooting case T-NoRB.

---

## P2. Label-selector placement (fleet-aware)

**Intent:** "Anywhere with `environment=production`."

```yaml
spec:
  placement:
    clusterAffinity:
      labelSelector:
        matchLabels:
          environment: production
```

**When:** fleet size > ~5 clusters, or when cluster membership changes frequently.
**Gotcha:** member clusters do not automatically inherit labels from anywhere. Labels
live on the `Cluster` object in the Karmada control plane and must be set explicitly
(`karmadactl join --cluster-label environment=production` or `kubectl label cluster member1 environment=production`).

---

## P3. Region-grouped placement with failover (multi-country)

**Intent:** "Prefer Singapore; fall back to Indonesia if SG capacity is short."

```yaml
spec:
  placement:
    clusterAffinities:
      - affinityName: primary-sg
        clusterNames: [sg-prod-1, sg-prod-2]
      - affinityName: backup-id
        clusterNames: [id-prod-1]
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: Weighted
      weightPreference:
        dynamicWeight: AvailableReplicas
```

**When:** regulated geographies, latency-sensitive frontends, multi-country compliance.
**Gotcha:** `clusterAffinities` (plural) and `clusterAffinity` (singular) are mutually
exclusive. The scheduler evaluates the plural form group-by-group in order, so list
preferred groups first.

---

## P4. Divided weighted replicas (cost-shaping)

**Intent:** "Run 60% in member1, 40% in member2 to match reserved-instance commitments."

```yaml
spec:
  placement:
    replicaScheduling:
      replicaSchedulingType: Divided
      replicaDivisionPreference: Weighted
      weightPreference:
        staticWeightList:
          - targetCluster: { clusterNames: [member1] }
            weight: 3
          - targetCluster: { clusterNames: [member2] }
            weight: 2
```

**When:** workloads with stable replica counts and clear cost preferences per cluster.
**Gotcha:** weights are integers, not percentages. The scheduler normalises by sum, so
{3, 2} and {30, 20} produce the same result; pick small integers for readability.

---

## P5. Duplicated replicas (HA frontends)

**Intent:** "Run the full replica count in *every* selected cluster, not split."

```yaml
spec:
  placement:
    replicaScheduling:
      replicaSchedulingType: Duplicated
```

**When:** stateless frontends behind a global load balancer where each region must
absorb full traffic during a sibling region's outage.
**Gotcha:** the underlying Deployment's `spec.replicas` is the per-cluster count, not
the global total. With 10 replicas and 3 selected clusters, you get 30 pods.

---

## P6. Image-registry override (per-region pull)

**Intent:** "member2 is in a network that cannot reach docker.io; use the local mirror."

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata: { name: image-mirror-id }
spec:
  resourceSelectors:
    - { apiVersion: apps/v1, kind: Deployment, name: global-frontend }
  overrideRules:
    - targetCluster: { clusterNames: [id-prod-1] }
      overriders:
        imageOverrider:
          - component: Registry
            operator: replace
            value: mirror.id.internal
```

**When:** air-gapped clusters, regional registry mirrors, registry vendor migration.
**Gotcha:** `imageOverrider` only works on built-in workloads (Pod, Deployment, ReplicaSet,
DaemonSet, StatefulSet, Job). For a custom CRD, fall back to `plaintext` with an explicit
JSON path. See `pkg/apis/policy/v1alpha1/override_types.go` for the supported list.

---

## P7. ConfigMap field override (per-region config)

**Intent:** "Use dark theme in member1, light theme in member2."

```yaml
overrideRules:
  - targetCluster: { clusterNames: [member2] }
    overriders:
      plaintext:
        - path: /data/theme
          operator: replace
          value: light-mode
```

**When:** feature flags, regional copy, currency symbols, locale-specific defaults.
**Gotcha:** the `path` is a JSON Pointer (RFC 6901), not a JSONPath expression. Slashes
inside keys must be escaped as `~1` and tildes as `~0`. Most users get this wrong on
their first try.

---

## P8. Dependency propagation (referenced ConfigMaps, Secrets)

**Intent:** "Propagate my Deployment along with the ConfigMaps and Secrets it references,
without listing them all in `resourceSelectors`."

```yaml
spec:
  propagateDeps: true
  resourceSelectors:
    - { apiVersion: apps/v1, kind: Deployment, name: global-frontend }
```

**When:** every real workload. Without this, the Deployment lands on the cluster but its
ConfigMap doesn't, and pods crash-loop on missing volume mounts.
**Gotcha:** dependency detection runs through `ResourceInterpreter`. For non-built-in
CRDs you must register an interpreter (Lua or Go) or `propagateDeps` is a no-op.

---

## P9. Conflict resolution (migration scenarios)

**Intent:** "There are already Deployments running in member clusters from before we
adopted Karmada. Take them over rather than refusing to propagate."

```yaml
spec:
  conflictResolution: Overwrite
```

**When:** migration onto Karmada, or recovery from a split-brain where two control planes
both wrote to the same member cluster.
**Gotcha:** the default is `Abort`. `Overwrite` is the right choice exactly *once* during
migration; leave it on permanently and you've defeated the safety net.

---

## P10. Priority and preemption (multi-team fleets)

**Intent:** "Team A's namespace policy must win over the org-wide default policy."

```yaml
spec:
  priority: 100
  preemption: Always
```

**When:** organisations where a central platform team ships defaults and product teams
need to override per-namespace.
**Gotcha:** without `preemption: Always`, once a resource is claimed by a lower-priority
policy a higher-priority one will *not* steal it. Priority alone is not enough.

---

## Anti-patterns the audit skill flags

- `resourceSelectors: []` — refused by validation (the webhook enforces MinItems=1) for
  security reasons (sensitive resources like Secret would otherwise be matched globally).
- `clusterAffinity` AND `clusterAffinities` set together — webhook refuses.
- Single OverridePolicy mixing JSON-patch rules for a Deployment *and* a ConfigMap in one
  `overriders` block — apply order is non-obvious and JSON paths will collide. Split into
  two `overrideRules` entries with separate `resourceSelectors`-style scoping, or split
  into two OverridePolicy objects.
- `replicaScheduling` placed at the root of `spec` instead of under `spec.placement` —
  the most common LLM hallucination; this is why `replicaScheduling` lives at
  `spec.placement.replicaScheduling`.
- Wildcard label selectors (`matchLabels: {}`) — admitted, but matches every cluster
  including ones you didn't intend.
