# karmada-debug-propagation

## Description
Debug why a Kubernetes resource in the Karmada control plane was not propagated
to one or more member clusters. Follows the propagation chain diagnostically.

## When to Load
- User says "resource not propagated", "not showing up in member cluster", "why isn't my Deployment in member2?"
- User says "propagation failed", "binding not created", "Work not applied".
- User is debugging any part of the Karmada propagation chain.

## Workflow

1. **Identify the resource and target cluster.**
   - What resource (Deployment, Service, etc.)?
   - Which namespace?
   - Which target cluster(s) is it supposed to go to?

2. **Walk the propagation chain step by step:**

   **Step 1: Does the resource exist in Karmada control plane?**
   ```bash
   kubectl get <kind> <name> -n <namespace>
   ```
   If No → Resource was never created or was deleted. Not a Karmada issue.

   **Step 2: Does a matching policy exist?**
   - For namespace-scoped resources: Check PropagationPolicy in the same namespace.
   - For cluster-scoped resources: Check ClusterPropagationPolicy.
   - Verify `resourceSelectors[].apiVersion` and `resourceSelectors[].kind` match exactly.
   - If `resourceSelectors[].name` is set, it must match the resource name exactly.
   - If `resourceSelectors[].labelSelector` is set, verify labels match.
   - If `resourceSelectors[].namespace` is set in a PropagationPolicy, verify it matches.
   - Check policy `priority` — higher priority policies may claim the resource first.

   **Step 3: Was a ResourceBinding (or ClusterResourceBinding) created?**
   ```bash
   kubectl get resourcebinding -n <namespace>
   kubectl get clusterresourcebinding
   ```
   Look for a binding whose `spec.resource` references the workload.
   If No → Policy selector mismatch, or binding controller error.
   Check: `kubectl logs -n karmada-system deployment/karmada-controller-manager`
   Check for labels: `propagationpolicy.karmada.io/permanent-id` and `propagationpolicy.karmada.io/namespace`/`name` annotations.

   **Step 4: Was the binding scheduled?**
   ```bash
   kubectl get resourcebinding <name> -n <ns> -o jsonpath='{.spec.clusters}'
   ```
   If `spec.clusters` is empty → check status conditions:
   ```bash
   kubectl get resourcebinding <name> -n <ns> -o jsonpath='{.status.conditions}'
   ```
   Reason codes:
   - `NoClusterFit` → placement constraints don't match any available cluster.
   - `Unschedulable` → clusters match but lack resources.
   - `SchedulerError` → internal error, check scheduler logs.
   - `QuotaExceeded` → FederatedResourceQuota limit reached.

   Investigate placement constraints:
   - `clusterAffinity.labelSelector` matches any clusters?
   - `clusterAffinity.fieldSelector` matches?
   - `clusterAffinity.exclude` excluding some?
   - `clusterTolerations` cover cluster taints?
   - `spreadConstraints` satisfiable?
   - Is scheduling suspended? `spec.suspension.scheduling=true`?

   **Step 5: Was a Work object created for the target cluster?**
   ```bash
   kubectl get work -n karmada-es-<cluster>
   ```
   If No → Check ResourceBinding `spec.suspension.dispatching` or `spec.suspension.dispatchingOnClusters`.
   Check execution controller logs.
   Verify the cluster is still joined and healthy: `kubectl get cluster <name>`.

   **Step 6: Was the Work applied?**
   ```bash
   kubectl get work <name> -n karmada-es-<cluster> -o jsonpath='{.status.conditions}'
   ```
   Check conditions:
   - `Applied=False` → resource not applied. Check `AppliedMessage` for error.
   - `Applied=True` → resource was applied.
   - Check if `Work.spec.suspendDispatching=true` — dispatching suspended at Work level.
   - Check `Work.spec.preserveResourcesOnDeletion` if relevant.

   Common Work application failures:
   - Conflict: Resource already exists on member cluster AND `conflictResolution=Abort`.
   - RBAC: Agent cannot create the resource on the member cluster.
   - CRD missing: The resource kind's CRD doesn't exist on the member cluster.

   **Step 7: Does the resource exist on the member cluster?**
   ```bash
   kubectl --context=<member-cluster> get <kind> <name> -n <namespace>
   ```
   If resource exists but is unhealthy → likely not a propagation issue but a workload issue.
   Check karmada-agent logs on the member cluster.

3. **Collect diagnostic output:**
   ```bash
   # Full diagnostic script:
   kubectl get propagationpolicy -A
   kubectl get resourcebinding -A
   kubectl get work -A
   kubectl get cluster
   ```

4. **Provide a failure point diagnosis** with the exact step where propagation stopped and the likely cause.

5. **Suggest remediation:**
   - Selector mismatch → fix `resourceSelectors` in the policy.
   - NoClusterFit → relax placement constraints or add more clusters.
   - Conflict → set `conflictResolution: Overwrite` or delete conflicting resource.
   - Suspended → remove suspension or set to `false`.
   - RBAC → grant necessary permissions to the karmada-agent service account.

## Knowledge References
- `hack/agent-skills/knowledge/troubleshooting.md`
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/components.md`
