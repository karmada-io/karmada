# karmada-audit-policy

## Description
Review Karmada PropagationPolicy and OverridePolicy YAML for correctness, including
selector mismatch detection, scope misuse, dependency issues, API version risks,
and best practice violations.

## When to Load
- User asks to "review this policy", "audit", "check my PropagationPolicy", "validate policy".
- User provides a policy YAML and asks "is this correct?".
- User asks "what's wrong with this policy?".
- User is about to apply a policy and wants pre-flight validation.

## Workflow

1. **Parse the policy YAML.**
   - Identify the Kind: PropagationPolicy, ClusterPropagationPolicy, OverridePolicy, ClusterOverridePolicy.
   - Extract `apiVersion`, `metadata`, `spec`.

2. **Run deterministic validation checks:**

   **A. Selector Checks:**
   - [ ] `resourceSelectors` is non-empty (required for PropagationPolicy, MinItems=1).
   - [ ] `apiVersion` and `kind` are well-formed (e.g., `apps/v1`, not `v1` for Deployment).
   - [ ] `name` is set? Then `labelSelector` is ignored — warn if both are set.
   - [ ] OverridePolicy with empty `resourceSelectors` matches ALL resources in namespace — confirm intentional.
   - [ ] Namespace-scoped policy (PropagationPolicy) cannot match cluster-scoped resources — check Kind.

   **B. Placement Checks:**
   - [ ] `clusterAffinity` and `clusterAffinities` should not both be set.
   - [ ] `spreadByField` and `spreadByLabel` should not both be set.
   - [ ] `clusterNames` reference valid cluster names.
   - [ ] `spreadConstraints.maxGroups >= minGroups`.

   **C. Dependency Checks:**
   - [ ] `propagateDeps: true` is set? Ensure referenced ConfigMaps/Secrets exist.
   - [ ] `dependentOverrides` list references policies that exist.

   **D. Failover Checks:**
   - [ ] Application failover configured but `propagateDeps` is false → warn that dependencies won't be migrated.
   - [ ] `purgeMode: Never` set — remind that manual cleanup will be needed.
   - [ ] `statePreservation` used → confirm `StatefulFailoverInjection` feature gate is enabled.

   **E. Override Checks:**
   - [ ] Override order: Image → Command → Args → Labels → Annotations → Field → Plaintext — warn about ordering if multiple types used.
   - [ ] `imageOverrider` without `predicate` applies to ALL images — confirm intentional.
   - [ ] `plaintext.path` uses correct JSON pointer syntax (e.g., `/spec/replicas` not `spec.replicas`).
   - [ ] `fieldOverrider.fieldPath` uses RFC 6901 syntax.
   - [ ] `overrideRules` used (correct) vs deprecated `targetCluster`/`overriders` (incorrect).

   **F. Scope Checks:**
   - [ ] ClusterPropagationPolicy targeting resources in `karmada-system`, `karmada-cluster`, `karmada-es-*` → invalid.
   - [ ] PropagationPolicy in namespace A with resourceSelector.namespace=B → may not work as expected.
   - [ ] ClusterOverridePolicy with `targetCluster` field (deprecated) → suggest `overrideRules`.

   **G. API Version Checks:**
   - [ ] Using `policy.karmada.io/v1alpha1` (correct).
   - [ ] Using legacy versions → warn.

   **H. Best Practice Checks:**
   - [ ] `conflictResolution` default is `Abort` — meaningful choice or oversight?
   - [ ] `priority` set to specific value — check for conflicts with other policies.
   - [ ] `preemption: Always` — understand implications.
   - [ ] `suspension` configured — confirm intentional.

3. **Categorize findings:**
   - **ERROR**: Will cause policy rejection or non-functional propagation.
   - **WARNING**: May cause unexpected behavior.
   - **INFO**: Best practice recommendation.

4. **Produce structured output:**
   ```markdown
   ## Audit Report: <Policy Name>

   ### Errors (must fix)
   - ...

   ### Warnings (should fix)
   - ...

   ### Info (consider)
   - ...
   ```

5. **If zero errors/warnings**, confirm the policy is syntactically and semantically valid.

## Knowledge References
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/troubleshooting.md`

## Template References
- `hack/agent-skills/templates/propagation-policy.yaml`
- `hack/agent-skills/templates/override-policy.yaml`
