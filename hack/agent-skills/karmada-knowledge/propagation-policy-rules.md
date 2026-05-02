# PropagationPolicy rules

This file is a dense rule set for AI agents that generate or review PropagationPolicy objects.

Treat these rules as mandatory unless a newer Karmada API document says otherwise.

## Core objective

A valid PropagationPolicy must precisely identify what resource templates are managed and how they are propagated across clusters.

Do not guess field meaning from generic Kubernetes behavior. Use the Karmada policy semantics below.

## Non-negotiable validity rules

- `spec.resourceSelectors` must be present and non-empty.
- An empty selector is not a match-all selector.
- Do not fabricate a policy that omits resourceSelectors.
- `apiVersion` and `kind` are required in each resource selector.
- `metadata.name` must be set on the policy.
- Use the correct namespace for namespaced policies.
- Do not use deprecated `association` when `propagateDeps` is the intended behavior.
- Do not invent unsupported fields.

## Resource selector semantics

A PropagationPolicy selects resource templates through `spec.resourceSelectors`.

Each ResourceSelector must obey these rules:
- `apiVersion` is required.
- `kind` is required.
- `namespace` is optional and defaults to the parent object scope.
- `name` is optional.
- `labelSelector` is optional.

Selection rules:
- If `name` is set, the selector targets that exact resource name.
- If `name` is set, `labelSelector` is ignored.
- If `name` is empty, the selector can use `labelSelector` to match resources by label.
- The selector is for identifying resource templates, not clusters.

Security rule:
- Never treat an omitted selector as meaning all resources of a kind.
- Never generate an empty resourceSelectors list.

## Dependency propagation

Use `propagateDeps` when the resource template has related objects that should follow it automatically.

Rules:
- `propagateDeps` is the modern mechanism.
- `association` is deprecated and should not be the primary choice.
- For workloads such as a Deployment, dependent ConfigMaps or Secrets may be propagated automatically when `propagateDeps` is true.
- If dependency propagation is enabled, do not require every referenced object to be listed manually in resourceSelectors unless the scenario explicitly needs that control.

Pitfall:
- Do not confuse dependency propagation with cluster placement or replica distribution. It only covers related resources.

## Placement vs ReplicaScheduling

These are different layers of intent.

Placement decides where the resource can run.
ReplicaScheduling decides how replicas are divided among the selected clusters.

Use this distinction strictly:
- Placement filters and prefers clusters.
- ReplicaScheduling applies only to resources with replicas in their spec.
- Placement does not define replica counts.
- ReplicaScheduling does not choose clusters by itself.

If the resource has no replica-bearing spec, ReplicaScheduling is usually unnecessary.

## Placement rules

Placement belongs in `spec.placement`.

Cluster selection rules:
- `clusterAffinity` and `clusterAffinities` are mutually exclusive.
- If both are omitted, any cluster can be a candidate.
- Use `clusterAffinity` for a single cluster group.
- Use `clusterAffinities` for ordered cluster groups.

ClusterAffinity semantics:
- `labelSelector` filters clusters by labels.
- `fieldSelector` filters clusters by fields.
- `clusterNames` selects explicit clusters.
- `exclude` removes clusters from consideration.

Field selector rules:
- Valid field keys are provider, region, and zone.
- Valid operators are In and NotIn.
- Do not invent arbitrary field keys.

Spread constraint rules:
- `spreadByField` and `spreadByLabel` are mutually exclusive.
- If both are omitted, spreadByField defaults to cluster.
- Available spread fields are cluster, region, zone, and provider.
- `minGroups` and `maxGroups` control cluster group cardinality.
- Keep spread constraints consistent with the intended blast radius and availability model.

Important behavior:
- ClusterAffinities are evaluated in order.
- If one group does not satisfy restrictions, the scheduler can fall through to the next group.
- If no group satisfies restrictions, scheduling fails.

## ReplicaScheduling rules

ReplicaScheduling belongs under Placement and only matters for replica-based workloads.

ReplicaSchedulingType:
- Duplicated means every candidate cluster gets the full original replica count.
- Divided means replicas are split among candidate clusters.
- Divided is the default behavior when the field is omitted.
- Do not set replica scheduling for objects that do not have replicas unless the policy design explicitly requires it.

ReplicaDivisionPreference:
- Aggregated divides replicas into as few clusters as possible while respecting available capacity.
- Weighted divides replicas according to weight preference.
- If Weighted is chosen and weightPreference is omitted, clusters are treated equally.

WeightPreference:
- Static weights are explicit and deterministic.
- Dynamic weight currently uses available replicas.
- If dynamic weight is set, static weights are ignored.

Pitfall:
- Do not confuse duplicated propagation with spreading work evenly. Duplicated means full replicas everywhere.

## Interaction rules

The agent must reason in this order:
1. identify the target resource template with resourceSelectors
2. determine cluster eligibility with placement
3. determine replica distribution with replicaScheduling if the workload has replicas
4. then consider secondary policy knobs such as priority, preemption, conflict resolution, and dependency propagation

Do not mix these layers.

## Common mistakes to avoid

- Empty resourceSelectors
- Using labelSelector and name together and expecting both to matter
- Using clusterAffinity and clusterAffinities together
- Using spreadByField and spreadByLabel together
- Treating Placement as a substitute for resource selection
- Treating ReplicaScheduling as a substitute for placement
- Using deprecated association instead of propagateDeps
- Assuming omitted fields mean match all resources
- Assuming omitted placement means zero clusters instead of any cluster
- Assuming Duplicated and Divided mean the same thing

## Output checklist for an AI agent

Before returning a generated PropagationPolicy, verify:
- resourceSelectors is present and non-empty
- every selector has apiVersion and kind
- name and labelSelector are not misused together
- placement is internally consistent
- clusterAffinity and clusterAffinities are not both set
- spread constraint fields are not conflicting
- replicaScheduling only appears when needed
- deprecated association is not used unless specifically required
- the final YAML matches the intended resource scope and cluster behavior

## Mental model

Think of PropagationPolicy as three decisions in order:
- what to manage
- where it may run
- how replicas are distributed

If any of those decisions is ambiguous, stop and resolve the ambiguity before generating YAML.
