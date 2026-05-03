# karmada-search

## Description
Search Karmada resources (policies, bindings, works) and optionally resources
across member clusters. Covers query patterns, label-based search, and the
karmada-search component for cross-cluster queries.

## When to Load
- User asks "find all PropagationPolicies", "search for work objects", "list bindings".
- User asks "find resources across all clusters", "search member clusters".
- User wants to locate specific Karmada objects by label, annotation, or field.
- User asks how to use `karmada-search` or the search API.

## Workflow

1. **Determine the search scope.**
   - Karmada control plane only? → Use `kubectl` with Karmada CRDs.
   - Across member clusters? → Use `karmada-search` component or per-cluster `kubectl`.

2. **For Karmada control plane searches:**

   **Find PropagationPolicies matching a resource:**
   ```bash
   kubectl get propagationpolicy -A
   kubectl get propagationpolicy -n <ns> -o yaml
   ```

   **Find OverridePolicies for a resource:**
   ```bash
   kubectl get overridepolicy -A
   kubectl get clusteroverridepolicy
   ```

   **Find ResourceBindings for a workload:**
   ```bash
   kubectl get resourcebinding -n <ns>
   kubectl get resourcebinding -n <ns> -o jsonpath='{.items[?(@.spec.resource.name=="<name>")]}'
   ```

   **Find Works targeting a specific cluster:**
   ```bash
   kubectl get work -n karmada-es-<cluster>
   ```

   **Find all bindings referencing a policy (via permanent-id label):**
   ```bash
   kubectl get resourcebinding -A -l propagationpolicy.karmada.io/permanent-id=<id>
   kubectl get clusterresourcebinding -A -l clusterpropagationpolicy.karmada.io/permanent-id=<id>
   ```

   **Find all resources claimed by a specific policy (via annotations):**
   ```bash
   kubectl get <kind> -A -o json | jq '.items[] | select(.metadata.annotations["propagationpolicy.karmada.io/name"]=="<policy-name>")'
   ```

3. **For cross-cluster searches (karmada-search):**

   If the `karmada-search` component is enabled, use the search API:

   ```bash
   # Search Deployments across all member clusters
   kubectl get --raw "/apis/search.karmada.io/v1alpha1/search/apis/apps/v1/deployments"

   # Search resources by namespace
   kubectl get --raw "/apis/search.karmada.io/v1alpha1/search/apis/v1/namespaces/<ns>/configmaps"
   ```

   **ResourceRegistry** defines which resources are cached for search:
   ```bash
   kubectl get resourceregistry
   ```

4. **Search by labels across control plane:**

   ```bash
   # Resources with skip-auto-propagation
   kubectl get ns -l namespace.karmada.io/skip-auto-propagation=true

   # Works with specific status
   kubectl get work -A -o json | jq '.items[] | select(.status.conditions[] | select(.type=="Applied" and .status=="False"))'
   ```

5. **Search by health status:**

   ```bash
   # Find unhealthy aggregated statuses across bindings
   kubectl get resourcebinding -A -o json | jq '.items[] | select(.status.aggregatedStatus[] | select(.health=="Unhealthy"))'
   ```

6. **Provide an explanation of search scope options:**
   - Control plane search: Policies, bindings, works, clusters — all via `kubectl`.
   - Cross-cluster search: `karmada-search` component with `ResourceRegistry`.
   - Per-cluster search: Direct `kubectl --context=<member>` queries.
   - Hybrid: Combine finding from bindings(status.aggregatedStatus) to narrow down which member clusters to query.

## Knowledge References
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/components.md`
