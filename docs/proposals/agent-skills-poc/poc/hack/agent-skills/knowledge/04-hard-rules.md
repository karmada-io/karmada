# Hard Rules — schema constraints an LLM must NOT violate

Each rule below has been chosen because (a) LLMs hallucinate it routinely and (b) the
correction is mechanical, so a deterministic helper can enforce it before the YAML ever
reaches the user. Skills that generate YAML must load this file. The audit skill scores
its findings against this list.

<karmada-hard-rules>

## R1. replicaScheduling lives under spec.placement, not at spec root.
WRONG: `spec.replicaScheduling.replicaSchedulingType`
RIGHT: `spec.placement.replicaScheduling.replicaSchedulingType`
Source: `pkg/apis/policy/v1alpha1/propagation_types.go` — `Placement` struct contains
`ReplicaScheduling`.

## R2. resourceSelectors is required and must have at least one item.
The webhook enforces `MinItems=1` for security reasons (an empty selector would otherwise
match Secrets and other sensitive resources globally). An LLM that emits
`resourceSelectors: []` will fail admission. Always emit at least one selector.

## R3. ResourceSelector.apiVersion is case-sensitive and includes the group.
WRONG: `apiVersion: Apps/v1` or `apiVersion: apps`
RIGHT: `apiVersion: apps/v1`
For core resources: `apiVersion: v1` (no group prefix).

## R4. clusterAffinity and clusterAffinities are mutually exclusive.
Pick one. The webhook rejects both being set. `clusterAffinities` (plural) is for
ordered fallback groups; `clusterAffinity` (singular) is the simple form.

## R5. Override `path` is a JSON Pointer, not JSONPath.
WRONG: `path: $.spec.template.spec.containers[0].image`
WRONG: `path: spec.template.spec.containers[0].image`
RIGHT: `path: /spec/template/spec/containers/0/image`
Slashes inside keys are escaped as `~1`, tildes as `~0` (RFC 6901).

## R6. imageOverrider only applies to known workload kinds.
Built-ins: Pod, ReplicaSet, Deployment, DaemonSet, StatefulSet, Job.
For any other kind, use `plaintext` with an explicit JSON Pointer path, or register a
custom interpreter. Skills should never emit `imageOverrider` for, say, an Argo
Rollout — it will silently no-op.

## R7. propagateDeps only works for resources with a registered interpreter.
For built-in workloads (Deployment, StatefulSet, ReplicaSet, Job, CronJob, DaemonSet)
the default interpreter discovers ConfigMaps, Secrets, PVCs, ServiceAccounts referenced
by the pod template. For custom CRDs the user must register a Lua or webhook interpreter
or `propagateDeps: true` is a silent no-op.

## R8. conflictResolution defaults to Abort. Overwrite must be deliberate.
Defaulting an LLM-generated policy to `Overwrite` is dangerous: it bulldozes existing
in-cluster state without confirmation. The skill must only emit `Overwrite` when the
user has explicitly described a migration scenario.

## R9. Priority is an integer where higher wins, but preemption is required to steal.
A new policy with `priority: 100` does not displace an existing claim from a `priority: 1`
policy unless the new policy also sets `preemption: Always`. Defaults: `priority=0`,
`preemption=Never`.

## R10. ClusterPropagationPolicy vs PropagationPolicy choice.
- PropagationPolicy: namespaced, matches resources in its own namespace (or cluster-scoped
  resources only when its own namespace is the policy's namespace).
- ClusterPropagationPolicy: cluster-scoped, can match resources across all namespaces
  AND can match cluster-scoped resources.
A namespaced workload usually wants PropagationPolicy. A CRD with cluster scope
(e.g., ClusterRole) requires ClusterPropagationPolicy.

## R11. Static weights must be integers, not floats or percentages.
WRONG: `weight: 0.6` or `weight: "60%"`
RIGHT: `weight: 6` (paired with another cluster's `weight: 4`)
The scheduler normalises by sum; readability beats precision.

## R12. labelSelector is ignored when name is set.
On a ResourceSelector, `name` takes precedence. Setting both is admitted but the
labelSelector is silently dropped. Skills generating selectors must pick one explicitly
and warn if both are present.

## R13. SchedulerName defaults to "default-scheduler".
Don't emit a custom value unless the user has confirmed a custom scheduler is registered.
A typo here means the binding sits unscheduled forever.

## R14. Don't emit deprecated fields.
- `spec.association` — deprecated in favor of `spec.propagateDeps`.
- `OverridePolicy.spec.targetCluster` + `spec.overriders` — deprecated since v1.0 in favor
  of `spec.overrideRules`.
- `purgeMode: Immediately` and `purgeMode: Graciously` — deprecated, use `Directly` and
  `Gracefully` respectively.

## R15. Mixed override targets need separate overrideRules entries.
An OverridePolicy targeting both a Deployment (which needs `/spec/template/.../image`)
and a ConfigMap (which needs `/data/...`) must split into two `overrideRules` entries,
each with its own `targetCluster` and `overriders`. Mixing them in one `overriders` block
guarantees one of the JSON paths will collide.

</karmada-hard-rules>

## How the generator + auditor consume this file

- The `karmada-create-policy` skill instructs the agent: "Before emitting YAML, validate
  against the constraints in `<karmada-hard-rules>`."
- The `karmada-audit-policy` skill instructs the agent: "For each rule R1-R15, check the
  given YAML and report a finding with rule id, severity (error/warn), and the offending
  path."
- The deterministic helpers in `scripts/` enforce R1, R2, R3, R5, R11, R14 mechanically
  without consulting an LLM. The remainder require semantic context that the LLM provides.
