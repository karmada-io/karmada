---
name: karmada-create-policy
description: Generate a syntactically valid PropagationPolicy (and optionally OverridePolicy) from a workload manifest and a placement intent stated in natural language. Use when the user asks for new policy YAML.
metadata:
  type: workflow
  loads:
    - knowledge/00-overview.md
    - knowledge/01-policy-patterns.md
    - knowledge/04-hard-rules.md
    - knowledge/10-propagation-schema.md
    - knowledge/11-override-schema.md
  uses_scripts:
    - scripts/generate_policy.py
  templates:
    - templates/intent.example.json
    - templates/propagationpolicy.minimal.yaml
    - templates/overridepolicy.minimal.yaml
---

# karmada-create-policy

The agent's job inside this skill is **not** to write the policy YAML itself — that
work is delegated to `scripts/generate_policy.py`, which is deterministic and cannot
hallucinate field names. The agent's job is to translate the user's English intent
into the structured JSON intent the script expects, then validate and explain the
result.

## Workflow

1. **Load context.** Read `knowledge/04-hard-rules.md` and the two schema files
   (10 and 11). The policy-patterns file (01) is the catalog of known shapes —
   reach for it when picking between placement modes.
2. **Extract intent.** From the user's message + any attached workload YAML, derive:
   - `policy_name` (kebab-case, derived from the workload's metadata.name)
   - `namespace` (the workload's namespace, or `default`)
   - `kind_scope` (cluster-scoped CRDs require `kind_scope: cluster`)
   - `resources[]` (one per `resourceSelectors` entry; use `name:` when the workload
     is named uniquely, fall back to `labelSelector` only when the user describes a
     fleet of similar workloads)
   - `placement.mode` — pick from `name-list`, `label-selector`, `ordered-groups`
   - `replica_strategy` — one of `duplicated`, `divided-weighted`, `divided-dynamic`,
     or omit if the user did not mention replica distribution
   - `propagate_deps` — default `true` for any Deployment / StatefulSet / Job because
     real workloads always reference ConfigMaps and Secrets
   - `overrides[]` — only when the user explicitly described per-cluster differences
3. **Call the generator.**
   ```
   echo '<intent json>' | python3 hack/agent-skills/scripts/generate_policy.py
   ```
   The script will refuse to emit YAML if the intent violates a hard rule. Treat any
   non-zero exit as a hint that your intent extraction missed something — fix the
   intent, do not bypass the script.
4. **Audit.** Pipe the generated YAML through `karmada-audit-policy` (sibling skill)
   and only show the user the result if the audit reports zero `error` findings.
5. **Explain.** Tell the user *why* you picked each shape. Cite the policy-patterns
   entry (e.g. "this is pattern P3 from `knowledge/01-policy-patterns.md`") and the
   schema row that justifies any non-obvious field.

## Decision table for placement.mode

| User says…                                 | mode             | typical follow-up question to confirm  |
|--------------------------------------------|------------------|----------------------------------------|
| names specific clusters                    | `name-list`      | did they spell the names correctly?    |
| mentions "production", "region=eu", labels | `label-selector` | do the Cluster objects carry those?    |
| mentions "fall back", "primary/secondary"  | `ordered-groups` | which group is preferred?              |
| dev/test fleet                             | `name-list`      | —                                      |

## Decision table for replica_strategy

| User says…                                 | replica_strategy    |
|--------------------------------------------|---------------------|
| nothing about replicas                     | omit                |
| "each cluster runs the full count"         | `duplicated`        |
| "60/40 split", "more in member1"           | `divided-weighted`  |
| "scale with available capacity"            | `divided-dynamic`   |

## When to ask the user a follow-up vs guess

Ask when:
- The user named no clusters AND no labels.
- The user implied an override but didn't say which cluster differs.
- The workload is a custom CRD (no built-in interpreter); confirm dependency propagation
  is wanted.

Guess when:
- The user named a workload that is clearly a Deployment / StatefulSet / Service —
  pick the obvious selector and move on.
- The user mentioned regions but a fleet view is in scope from prior turns.

## Hard rules the script enforces (R-numbers map to knowledge/04-hard-rules.md)

R1, R2, R3, R5, R11, R14 are enforced by the script. R4, R6, R7, R8, R9, R10, R12, R13,
R15 are semantic and require the agent's judgment. Always re-read R8 and R10 before
producing YAML — they are the two rules whose violation produces a *plausible-looking*
but wrong policy.

## Failure modes to anticipate

- **User pastes a workload but no placement intent.** Ask one question:
  "Which clusters or regions should this propagate to?"
- **User asks for `Overwrite` mode "to keep things simple".** Push back. Cite R8.
  `Overwrite` is a migration tool, not a default.
- **User pastes a custom CRD and says "propagate this with its dependencies".** Warn
  that `propagateDeps` is a no-op without a registered ResourceInterpreter. Quote R7.

## Example end-to-end

User: *"Generate a Karmada policy for this Deployment, place it in member1 and member2,
use the image registry mirror.id.internal in member2."*

Agent builds intent:
```json
{
  "policy_name": "global-frontend",
  "namespace": "default",
  "resources": [{"apiVersion": "apps/v1", "kind": "Deployment", "name": "global-frontend"}],
  "placement": {"mode": "name-list", "clusters": ["member1", "member2"]},
  "propagate_deps": true,
  "overrides": [{
    "target_clusters": ["member2"],
    "image": {"component": "Registry", "operator": "replace", "value": "mirror.id.internal"}
  }]
}
```

Agent runs the script. The audit reports zero errors. Agent shows the YAML with a
two-sentence note: "I used pattern P1 (name-list placement) and P6 (image-registry
override). `propagateDeps` is on because Deployments usually reference ConfigMaps."
