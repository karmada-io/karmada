# Handling Multi-Path Dependency Conflicts and Best Practices

This guide explains how Karmada handles dependency distribution when a dependent resource (e.g., ConfigMap/Secret) is referenced by multiple parents, and how to troubleshoot and optimize such scenarios.

Scope:

- Applies when dependency auto-propagation is enabled (PropagateDeps) and the Dependencies Distributor creates dependency-generated ResourceBindings (RBs).
- "Dependency-generated" RBs are the ones created for dependencies and have `spec.placement == nil`.

## Policy Aggregation Rules

When multiple parent RBs/CRBs reference the same dependency, their relevant policies are aggregated for the dependency-generated RB as follows:

- ConflictResolution: If any parent sets `Overwrite`, aggregated is `Overwrite`; otherwise `Abort`.
- PreserveResourcesOnDeletion: If any parent sets `true`, aggregated is `true`; otherwise `false`.

Re-aggregation occurs when:

- Parent list changes (add/remove parents).
- Parent RB/CRB policy fields change.
- The dependency relationship changes due to workload spec updates.

To avoid churn, updates are debounced and only applied when effective values change.

## Explicit Governance Takes Precedence

If a dependency RB is explicitly governed (e.g., has `spec.placement` set or managed by an explicit policy), the Dependencies Distributor will not override conflict-related fields via the dependency path. In such cases, you may observe the event:

- DependencyOverriddenByExplicitPolicy: dependency path settings are ignored because an explicit policy governs the RB.

## Events Reference

To improve observability, the following events are emitted:

- DependencyPolicyAggregated (Normal): Aggregated dependency policies have been applied to the dependency-generated RB.
- DependencyPolicyConflict (Warning): Parents specify divergent policies. Aggregation still applies but signals potential risk.
- DependencyOverriddenByExplicitPolicy (Warning): The explicit policy takes precedence over dependency path settings.
- DependencyMultiParentDetected (Warning): Multi-parent relationship detected; consider merging policies or simplifying dependencies.
- DependencyMultiParentBlocked (Warning): Multi-parent relationship is forbidden by FeatureGate and the action was blocked.

## Governance: Forbid Multi-Parent (Optional)

FeatureGate: `ForbidMultiDepPropagation` (alpha, default=false).

- When enabled: Blocks creating or updating dependency-generated RBs that would result in multiple parents. Clear events and errors are emitted with remediation guidance.
- When disabled (default): Multi-parent relationships are allowed but a warning event is emitted.

Enable the FeatureGate on karmada-controller-manager:

- `--feature-gates=ForbidMultiDepPropagation=true`

## Troubleshooting Steps

1. Inspect Events

   - `kubectl describe resourcebinding <ns>/<name>`
   - Look for events listed above to identify conflicts, overrides, or multi-parent detections.

2. Check Parent Relationships

   - Inspect `spec.requiredBy` in the dependency RB to see all parents.
   - Verify each parent RB/CRB `spec.conflictResolution` and `spec.preserveResourcesOnDeletion`.

3. Validate Aggregation Outcome

   - Confirm the aggregated `spec.conflictResolution` and `spec.preserveResourcesOnDeletion` match the rules.
   - If the RB is explicitly governed, expect dependency path settings to be ignored.

4. Remediate Multi-Parent Conflicts
   - Prefer merging desired settings into a single PropagationPolicy/ClusterPropagationPolicy so the dependency has a single parent.
   - Or simplify dependency relationships to avoid multi-parent reference paths.
   - If strict governance is desired, enable `ForbidMultiDepPropagation` to hard-block multi-parent.

## Operational Best Practices

- Keep dependency graphs simple; avoid unnecessary multi-parent references.
- Consolidate policies where possible to reduce aggregation ambiguity.
- Monitor events and logs regularly to detect conflicts early.
- Use the FeatureGate to gradually enforce governance after observing the impact.

## Observability Notes

- Logs include aggregation summaries for debugging, e.g. with a prefix like `[dep-agg]` indicating parents count, aggregated results and parent details.
- Metrics (if enabled in your build):
  - `karmada_dependencies_dependency_policy_conflicts_total{namespace}`: count of detected policy conflicts.
  - `karmada_dependencies_dependency_parents_count{namespace}`: histogram of parent counts observed during aggregation.

No CRD changes are required for these capabilities. The default behavior is backward-compatible and remains unchanged unless the FeatureGate is explicitly enabled.
