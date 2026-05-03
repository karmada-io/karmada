# karmada-create-policy

## Description
Generate Karmada PropagationPolicy and OverridePolicy YAML for a given workload, including
placement rules, replica scheduling, and per-cluster overrides.

## When to Load
- User asks to "create a Karmada policy", "generate a PropagationPolicy", "write an OverridePolicy".
- User describes placement intent: "place this Deployment in member1 and member2".
- User asks for "multi-cluster deployment" or "propagate to clusters".
- User wants to "use different image registry in cluster X".
- User asks "how do I spread replicas across regions".

## Workflow

1. **Identify the workload.**
   - Ask for or determine: `apiVersion`, `kind`, `name`, `namespace`.
   - If the resource is cluster-scoped (e.g., ClusterRole, CRD), use `ClusterPropagationPolicy`.

2. **Determine placement requirements.**
   - Specific cluster names? → `clusterAffinity.clusterNames`
   - Cluster labels? → `clusterAffinity.labelSelector`
   - Cluster fields (region, zone, provider)? → `clusterAffinity.fieldSelector`
   - Spread across regions/zones? → `spreadConstraints`
   - Need overflow/backup clusters? → `clusterAffinities` with `overflowAffinities`

3. **Determine replica scheduling (for workloads with replicas).**
   - Same replica count everywhere? → `replicaSchedulingType: Duplicated`
   - Divide replicas across clusters? → `replicaSchedulingType: Divided`
   - Equal weight? → omit `weightPreference`
   - Custom weights? → `weightPreference.staticWeightList`
   - Dynamic by available resources? → `weightPreference.dynamicWeight: AvailableReplicas`

4. **Determine override requirements.**
   - Image changes per cluster? → `imageOverrider`
   - Label/annotation differences? → `labelsOverrider` / `annotationsOverrider`
   - Command/args changes? → `commandOverrider` / `argsOverrider`
   - Arbitrary field changes? → `plaintext`
   - Structured field changes? → `fieldOverrider`

5. **Generate the YAML.**
   - Use `hack/agent-skills/templates/propagation-policy.yaml` as starter.
   - Use `hack/agent-skills/templates/override-policy.yaml` as starter.
   - Fill in all fields based on user requirements.
   - Always use `apiVersion: policy.karmada.io/v1alpha1`.
   - Include informative `metadata.name` (e.g., `<workload>-propagation`).

6. **Validate against the API schema:**
   - `resourceSelectors` must have at least one entry (MinItems=1).
   - `clusterAffinity` and `clusterAffinities` are mutually exclusive.
   - `spreadByField` and `spreadByLabel` are mutually exclusive.
   - `dispatching` and `dispatchingOnClusters` are mutually exclusive.

7. **Present the output** with a brief explanation of what each section does.

## Knowledge References
- `hack/agent-skills/knowledge/policy-apis.yaml`
- `hack/agent-skills/knowledge/policy-patterns.md`

## Template References
- `hack/agent-skills/templates/propagation-policy.yaml`
- `hack/agent-skills/templates/cluster-propagation-policy.yaml`
- `hack/agent-skills/templates/override-policy.yaml`
- `hack/agent-skills/templates/cluster-override-policy.yaml`

## Example References
- `hack/agent-skills/examples/image-override/`
- `hack/agent-skills/examples/multi-country-setup/`
- `hack/agent-skills/examples/failover/`
