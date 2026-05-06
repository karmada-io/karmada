---
name: karmada-knowledge
description: Foundational Karmada concepts — control plane, member clusters, propagation, overrides — loaded before any other Karmada skill so the agent can reason about multi-cluster deployments accurately.
when_to_load: |
  Trigger on user mentions of: "karmada", "propagation policy", "override policy",
  "member cluster", "karmada-controller-manager", "ResourceBinding", "Work",
  "execution namespace", or any multi-cluster Kubernetes question where the
  repository contains Karmada manifests.
---

# karmada-knowledge

Use this skill **first** whenever Karmada is in scope. It anchors all other
skills (`karmada-create-policy`, `karmada-audit-policy`, …) in a shared mental
model so generated artifacts are internally consistent.

## Core mental model

Karmada is a Kubernetes control plane that **propagates** workloads from a
single host control plane out to many **member clusters**.

```
┌──────────────────────┐    PropagationPolicy
│  Karmada control     │  ─────────────────────►   ┌───────────┐
│  plane (host)        │                           │ member-1  │
│                      │    OverridePolicy         ├───────────┤
│  user creates a      │  ─────────────────────►   │ member-2  │
│  Deployment here     │                           ├───────────┤
└──────────────────────┘                           │ member-N  │
                                                   └───────────┘
```

A user submits a regular Kubernetes resource (e.g., `Deployment`) to the
Karmada apiserver. A **`PropagationPolicy`** matches it via `resourceSelectors`
and decides *which* clusters receive it. An **`OverridePolicy`** mutates the
manifest *per cluster* (e.g., swap the image registry in China).

## Object lifecycle the agent must keep straight

1. User applies `Deployment` + `PropagationPolicy` to the Karmada apiserver.
2. `karmada-controller-manager` creates a **`ResourceBinding`** that records
   the scheduling decision.
3. `karmada-scheduler` picks target clusters and writes them onto the binding.
4. The execution controller materializes one **`Work`** per target cluster in
   the cluster's *execution namespace* (`karmada-es-<cluster-name>`).
5. The `karmada-agent` (or push-mode executor) syncs the `Work` payload into
   the member cluster as the real `Deployment`.

When propagation fails, the agent must inspect this chain in **this exact
order**: PropagationPolicy → ResourceBinding → Work → member cluster.

## Field cheat sheet

See [`knowledge/policy-apis.yaml`](../../knowledge/policy-apis.yaml) for the
full field map. The fields agents most often get wrong:

| Field                           | Common mistake                             | Correct usage |
|---------------------------------|--------------------------------------------|---------------|
| `placement.clusterAffinity`     | confused with `clusterAffinities` (plural) | Singular = one group; plural = ordered list with failover |
| `placement.replicaScheduling.replicaSchedulingType` | omitted; defaults silently | Set to `Duplicated` *or* `Divided` explicitly |
| `resourceSelectors[].apiVersion`| using `v1` for Deployments                 | Must be `apps/v1` |
| `spreadConstraints[].spreadByField` | mixed with `spreadByLabel` in same constraint | They are mutually exclusive in one entry |
| `override.plaintext[].path`     | written as dotted path (`spec.replicas`)   | Must be JSON Pointer (`/spec/replicas`) |

## Idioms to suggest by default

- For region-aware spread, prefer `spreadConstraints` with
  `spreadByField: region` over hand-listing cluster names — see
  [`knowledge/policy-patterns.md`](../../knowledge/policy-patterns.md).
- For image-registry overrides, prefer `imageOverrider` (with `-r`)
  over a raw JSON-Pointer plaintext patch — it survives image tag bumps.

## Skill workflow

1. Read the user's request and identify the Karmada object(s) involved.
2. Load this skill's knowledge files (`policy-apis.yaml`, `policy-patterns.md`).
3. Hand off to a downstream skill (`karmada-create-policy`,
   `karmada-debug-propagation`, …) with the established context.
4. Never invent fields. If the user asks for behavior not covered here,
   say so and link the upstream API source under `pkg/apis/policy/v1alpha1/`.

## References

- API source of truth: `pkg/apis/policy/v1alpha1/propagation_types.go`,
  `override_types.go`
- Knowledge base: `hack/agent-skills/knowledge/`
