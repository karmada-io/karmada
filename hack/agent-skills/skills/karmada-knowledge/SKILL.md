# karmada-knowledge

## Description
Load Karmada architecture concepts, API types, component relationships, policy patterns,
and well-known conventions. This is the foundational skill all other Karmada skills depend on.

## When to Load
- User mentions "Karmada", "multi-cluster", "propagation", "override policy", "resource binding" for the first time.
- User asks about Karmada's architecture, components, or API structure.
- User asks how Karmada works or what Karmada can do.
- Any other Karmada skill is triggered (load this first as prerequisite).

## Workflow

1. **Assess the user's Karmada knowledge level.**
   - If they use precise terms (PropagationPolicy, ResourceBinding, Work), provide targeted information.
   - If they use vague terms (spread to clusters, multi-region), start with architectural overview.

2. **Load relevant knowledge files based on the query:**
   - `hack/agent-skills/knowledge/policy-apis.yaml` for API field references, types, enums.
   - `hack/agent-skills/knowledge/policy-patterns.md` for common policy configurations.
   - `hack/agent-skills/knowledge/components.md` for Karmada component architecture.
   - `hack/agent-skills/knowledge/troubleshooting.md` for debug playbooks.

3. **Explain the propagation chain** when relevant:
   ```
   Resource Template → Policy Match → ResourceBinding → Scheduling → Work → Member Cluster
   ```

4. **Surface the four core CRDs and their relationship:**
   - `PropagationPolicy` / `ClusterPropagationPolicy` — what to propagate and where.
   - `OverridePolicy` / `ClusterOverridePolicy` — how to modify per cluster.
   - `ResourceBinding` / `ClusterResourceBinding` — the intermediate binding object.
   - `Work` — the per-cluster execution unit.

5. **Clarify scope rules:**
   - `PropagationPolicy` is Namespace-scoped, only matches resources in its namespace.
   - `ClusterPropagationPolicy` is Cluster-scoped, can match cluster-scoped resources and resources in any namespace except system namespaces (`karmada-system`, `karmada-cluster`, `karmada-es-*`).
   - Same scoping applies to OverridePolicy vs ClusterOverridePolicy.

6. **Provide the correct API version:** Always use `policy.karmada.io/v1alpha1` for policies and `work.karmada.io/v1alpha2` (storage) for bindings.

## Knowledge References
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/policy-patterns.md`
- `hack/agent-skills/knowledge/components.md`
- `hack/agent-skills/knowledge/troubleshooting.md`
