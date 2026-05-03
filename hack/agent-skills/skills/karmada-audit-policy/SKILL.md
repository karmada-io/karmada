---
name: karmada-audit-policy
description: Review a Karmada PropagationPolicy / OverridePolicy YAML for correctness, schema bugs, and idiom violations using the deterministic validate-policy.py linter — never by free-reading.
when_to_load: |
  Trigger when the user asks to "review", "audit", "check", "lint", "is this
  policy correct", or pastes a Karmada policy YAML for inspection. Always
  loads `karmada-knowledge` first.
---

# karmada-audit-policy

Auditing policy YAML by eye is exactly how humans introduce bugs in the
first place. This skill forces the agent to call the linter, then
*explain* the findings — never to substitute its own opinion for a
finding the linter did not raise.

## Hard rules

1. **Always run the linter first.** Output goes into the response verbatim.
2. **Do not "interpret" missing findings.** If the linter is silent on a
   field, the policy is presumed correct on that axis.
3. **Each finding has a stable rule ID** (e.g., `KMD221`). Cite it.
   This makes audits diffable across runs.

## Workflow

### Step 1 — run the linter

```bash
python3 hack/agent-skills/scripts/validate-policy.py path/to/policy.yaml
# Or, for machine consumption:
python3 hack/agent-skills/scripts/validate-policy.py --format json policy.yaml
```

Exit codes:

- `0` — clean, no findings.
- `1` — at least one finding (errors and/or warnings).
- `2` — usage error or unparseable YAML.

### Step 2 — group findings by severity

Report in this order:

1. **error** — schema- or behavior-breaking. The policy will not work.
2. **warning** — works but contradicts a documented idiom.
3. **info** — opportunity to tighten.

For each finding, quote the rule ID, the JSON path, and the message
verbatim, then add a one-sentence "what to change" suggestion grounded
in `knowledge/policy-patterns.md`.

### Step 3 — if clean, do NOT invent findings

Say so plainly: *"Linter clean — no schema or idiom violations. Manual
review notes follow only if the user asks."* Then stop.

### Step 4 — propose a fixed policy

If errors exist, **regenerate** the corrected YAML using
`policy-generator.py` rather than hand-patching. Hand-patches reintroduce
the very bugs the audit caught.

## Rule reference (excerpt)

| Rule    | Severity | What it catches |
|---------|----------|-----------------|
| KMD001  | error    | wrong `apiVersion` (not `policy.karmada.io/v1alpha1`) |
| KMD011  | error    | `PropagationPolicy` missing `metadata.namespace` |
| KMD012  | error    | `Cluster*Policy` has `metadata.namespace` (cluster-scoped) |
| KMD025  | error    | `Deployment` selector with `apiVersion: v1` |
| KMD100  | error    | both `clusterAffinity` and `clusterAffinities` set |
| KMD101  | warning  | placement matches no clusters (no affinity, no spread) |
| KMD110  | error    | `replicaSchedulingType` not `Duplicated`/`Divided` |
| KMD120  | error    | `spreadByField` and `spreadByLabel` in same constraint |
| KMD131  | error    | `failover.application` without `tolerationSeconds` |
| KMD210  | error    | typo `imageOverride` instead of `imageOverrider` |
| KMD221  | error    | `plaintext.path` is JSONPath, not JSON Pointer |

The full set is the source of truth in `scripts/validate-policy.py`.

## Example session

```
$ python3 hack/agent-skills/scripts/validate-policy.py bad.yaml
bad.yaml: [ERROR] KMD025 at $.spec.resourceSelectors[0].apiVersion: Deployment requires apiVersion 'apps/v1', not 'v1'
bad.yaml: [ERROR] KMD221 at $.spec.overrideRules[0].overriders.plaintext[0].path: path must be a JSON Pointer (RFC 6901) — looks like a JSONPath; rewrite as JSON Pointer (e.g., /spec/replicas)
```

Agent response:

> 2 errors:
>
> - **KMD025** — `resourceSelectors[0].apiVersion` is `v1` for a
>   `Deployment`. Change to `apps/v1`.
> - **KMD221** — `plaintext[0].path` is `spec.replicas` (JSONPath).
>   Rewrite as JSON Pointer: `/spec/replicas`.
>
> Regenerate with `policy-generator.py --kind OverridePolicy
> --override-replicas <N> ...` to avoid hand-editing.
