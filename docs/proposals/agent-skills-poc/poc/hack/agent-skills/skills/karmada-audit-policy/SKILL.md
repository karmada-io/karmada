---
name: karmada-audit-policy
description: Review a Karmada PropagationPolicy or OverridePolicy before the user applies it. Returns structured findings keyed to the hard-rules catalog. Use when the user asks "is this policy correct?", "what's wrong with this YAML?", or pastes a policy and asks for review.
metadata:
  type: workflow
  loads:
    - knowledge/04-hard-rules.md
    - knowledge/10-propagation-schema.md
    - knowledge/11-override-schema.md
  uses_scripts:
    - scripts/audit_policy.py
---

# karmada-audit-policy

This skill answers one question: **should the user apply this YAML?**

The agent does not eyeball the YAML — it pipes the YAML through
`scripts/audit_policy.py`, reads the structured findings, and adds the *semantic* checks
that the script cannot do (e.g. "this label selector exists but no Cluster in your
fleet actually carries that label").

## Workflow

1. Save the user's YAML to a temp file or feed it via stdin to the linter:
   ```
   python3 hack/agent-skills/scripts/audit_policy.py /tmp/user-policy.yaml --format json
   ```
2. Read the JSON. For each finding, look up the rule id (R1–R15) in
   `knowledge/04-hard-rules.md` and cite the rule when explaining the finding to the
   user.
3. Add semantic findings the linter cannot detect:
   - **Selector vs intent mismatch** — the YAML targets `app=foo` but the user said
     they wanted "the frontend". Are those the same?
   - **Cluster names vs reality** — if the user has already shared a fleet view in
     this conversation, flag clusterAffinity entries that reference unknown clusters.
   - **Risk in context** — `propagateDeps: false` on a Deployment with `envFrom` is
     a soft warning. The linter only flags hard rules; the agent owns soft signal.
4. Present results in two sections: "Must fix" (errors) and "Worth thinking about"
   (warns + agent-added semantic findings). Never collapse them into one list — the
   user needs to know what blocks apply vs what to consider.

## Output shape

When the linter returns no errors:

```
✅ No hard-rule violations.

Worth thinking about:
- [R8/warn] spec.conflictResolution is Overwrite — only use during migration.
- [semantic] Your placement labels environment=production, but cluster member3 in
  this thread is labeled environment=staging. It will not be selected.
```

When the linter returns errors:

```
❌ Do not apply yet.

Must fix:
- [R1/error] spec.replicaScheduling is at spec root; move under spec.placement.
- [R5/error] override path "$.spec.template.spec.containers[0].image" is JSONPath;
  the correct JSON Pointer is "/spec/template/spec/containers/0/image".

Worth thinking about: …
```

## When NOT to add semantic findings

If the conversation has no fleet view, no workload manifest, no override intent
stated, the agent cannot add semantic findings — and should say so explicitly:
"Linter is clean. I don't have your fleet labels or the source workload in this
thread, so I can't tell whether the selectors match reality."

## Failure modes

- **User pastes Markdown / kubectl output, not YAML.** Strip fences and re-parse.
  If parsing still fails, ask for raw YAML.
- **User pastes multiple documents.** The linter handles `safe_load_all`; flag
  cross-document issues (e.g. an OverridePolicy whose `resourceSelectors` do not
  intersect the PropagationPolicy's selectors).
- **User asks for a "deeper" audit.** Offer to run `kubeconform` against the CRD
  for full OpenAPI validation; that is out of scope for the in-repo helper but
  trivial to add as a follow-up.

## What the linter checks (rule → script function)

| Rule | What the linter checks                                                       |
|------|------------------------------------------------------------------------------|
| R1   | `spec.replicaScheduling` not at spec root                                    |
| R2   | `spec.resourceSelectors` is non-empty                                        |
| R3   | apiVersion case / format                                                     |
| R4   | clusterAffinity vs clusterAffinities mutual exclusivity                      |
| R5   | OverridePolicy `path` is JSON Pointer, not JSONPath                          |
| R8   | warn on conflictResolution=Overwrite                                         |
| R9   | priority > 0 without preemption=Always (info)                                |
| R11  | static weights are integers                                                  |
| R12  | name + labelSelector both set (warn: labelSelector silently ignored)         |
| R14  | deprecated fields (association, targetCluster+overriders, Immediately, etc.) |

R6, R7, R10, R13, R15 are semantic and remain the agent's responsibility.
