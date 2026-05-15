# Progressive Rollout — Sample Manifests

Static YAML manifests used in the [Progressive Rollout Strategies[tutorial] tutorial.

The tutorial demonstrates three rollout strategies across multiple clusters using only
Karmada `PropagationPolicy` and `OverridePolicy` — no additional controllers required.

## Files

### Base application

| File | Purpose |
|------|---------|
| `http-probe-app.yaml` | Base app at `v1`, `replicaSchedulingType: Divided` (2 replicas per cluster) |
| `http-probe-app-v2.yaml` | Base app at `v2`, `replicaSchedulingType: Divided` — used to finalize Strategies 1 and 2 |

### Ingress-nginx

| File | Purpose |
|------|---------|
| `ingress-nginx-deploy.yaml` | ingress-nginx controller workload |
| `ingress-nginx-propagation.yaml` | `PropagationPolicy` for namespace-scoped ingress-nginx resources |
| `ingress-nginx-cluster-propagation.yaml` | `ClusterPropagationPolicy` for cluster-scoped ingress-nginx resources |

### Strategy 1 — Side-by-Side Testing (Canary)

| File | Purpose |
|------|---------|
| `http-probe-app-canary-member1.yaml` | Canary `Deployment` + `PropagationPolicy` targeting `member1` only |
| `http-probe-app-canary-member2.yaml` | Canary `Deployment` + `PropagationPolicy` targeting `member2` only |
| `http-probe-app-canary-member3.yaml` | Canary `Deployment` + `PropagationPolicy` targeting `member3` only |
| `http-probe-app-canary-all.yaml` | Single canary `Deployment` + `PropagationPolicy` targeting all three clusters |
| `http-probe-promote-override-member1.yaml` | `OverridePolicy` promoting the base to `v2` on `member1` |
| `http-probe-promote-override-member2.yaml` | `OverridePolicy` promoting the base to `v2` on `member2` |
| `http-probe-promote-override-member1-member2.yaml` | `OverridePolicy` promoting the base to `v2` on `member1` and `member2` |
| `http-probe-promote-override-all.yaml` | `OverridePolicy` promoting the base to `v2` on all three clusters |

### Strategy 2 — In-place Updates (Rolling Upgrade)

| File | Purpose |
|------|---------|
| `http-probe-rolling-upgrade-member1.yaml` | `OverridePolicy` rolling the base to `v2` on `member1` |
| `http-probe-rolling-upgrade-member1-member2.yaml` | `OverridePolicy` rolling the base to `v2` on `member1` and `member2` |
| `http-probe-rolling-upgrade-all.yaml` | `OverridePolicy` rolling the base to `v2` on all three clusters |

### Strategy 3 — Percentage-based Shifting (Wave Rollout)

| File | Purpose |
|------|---------|
| `http-probe-app-wave-base.yaml` | Base app at `v1`, `replicaSchedulingType: Duplicated` (6 replicas per cluster) — required for meaningful percentage steps |
| `http-probe-app-wave-base-v2.yaml` | Same as above but at `v2` — applied by `hack/wave-rollout.sh finalize` |
| `http-probe-app-wave-deployment.yaml` | Wave `Deployment` at `v2` — created by `hack/wave-rollout.sh start` |

The wave rollout is orchestrated by [`hack/wave-rollout.sh`](../../hack/wave-rollout.sh).
The `OverridePolicy` resources that scale the base down and the wave up are generated
dynamically by the script at runtime, since their replica counts depend on the target
percentage and the `--wait` interval supplied by the operator.

## Usage

See the [Progressive Rollout Strategies with Karmada][tutorial] tutorial for full
step-by-step instructions including inline YAML, dashboard observations, and sequence
diagrams for each strategy and demo.

[tutorial]: https://karmada.io/docs/tutorials/rollout/overview
