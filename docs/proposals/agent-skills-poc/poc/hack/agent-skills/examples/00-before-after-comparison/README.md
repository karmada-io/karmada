# Before vs After — generic LLM vs `karmada-create-policy`

This is the single artifact most worth showing a reviewer. Same workload. Same
natural-language request. Two outputs.

The "without skill" YAML is a representative output a stock LLM produces for this
prompt today. The "with skill" YAML is what the deterministic generator emits when
the same LLM has loaded `karmada-create-policy` and produced the structured
intent it expects.

## The request

> *"Make a Karmada policy for this Deployment. Place it on `member1` and `member2`.
> Use the image registry `mirror.id.internal` on `member2` only."*

The user attached a Deployment with `replicas: 6` that pulls config from a
`ConfigMap` via `envFrom`. See [`prompt.md`](prompt.md) for the full input.

## Without the skill (stock LLM)

[`without-skill.yaml`](without-skill.yaml) — 47 lines, looks plausible, **does not work**.
The auditor flags four rule violations and there is one further semantic bug it cannot
catch.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: global-frontend
  namespace: default
spec:
  resourceSelectors:
    - apiVersion: Apps/v1                    # ✗ R3 — group must be lowercase
      kind: Deployment
      name: global-frontend
  association: true                          # ✗ R14 — deprecated; use propagateDeps
  replicaScheduling:                         # ✗ R1 — must live under spec.placement
    replicaSchedulingType: Divided
    replicaDivisionPreference: Weighted
    weightPreference: { ... }
  placement:
    clusterAffinity:
      clusterNames: [member1, member2]
---
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: global-frontend-override
  namespace: default
spec:
  resourceSelectors:
    - apiVersion: Apps/v1                    # ✗ R3 again
      kind: Deployment
      name: global-frontend
  overrideRules:
    - targetCluster: {clusterNames: [member2]}
      overriders:
        plaintext:
          - path: $.spec.template.spec.containers[0].image   # ✗ R5 — JSONPath, not JSON Pointer
            operator: replace
            value: mirror.id.internal/library/nginx:1.27
```

### What's wrong

| Issue                                               | Rule | Severity | Caught by  |
|-----------------------------------------------------|------|----------|------------|
| `Apps/v1` (capitalised group)                       | R3   | error    | linter     |
| `spec.replicaScheduling` at spec root               | R1   | error    | linter     |
| `spec.association: true` (deprecated)               | R14  | warn     | linter     |
| Override `path` is JSONPath, not JSON Pointer       | R5   | error    | linter     |
| **No `propagateDeps: true`** — the ConfigMap will not propagate and pods will crash-loop with missing env | semantic | — | skill judgment |

The audit JSON for this file: [`audit-findings.json`](audit-findings.json).

```
3 errors, 1 warn — do not apply.
```

## With the skill (`karmada-create-policy` loaded)

[`with-skill.yaml`](with-skill.yaml) — 26 lines, structurally clean, audit returns zero
findings.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: global-frontend
  namespace: default
spec:
  resourceSelectors:
  - apiVersion: apps/v1                      # ✓ lowercase group
    kind: Deployment
    name: global-frontend
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
  propagateDeps: true                        # ✓ ConfigMap rides along
---
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: global-frontend-override
  namespace: default
spec:
  resourceSelectors:
  - apiVersion: apps/v1
    kind: Deployment
    name: global-frontend
  overrideRules:
  - targetCluster:
      clusterNames:
      - member2
    overriders:
      imageOverrider:                        # ✓ structured imageOverrider, no JSON pointer needed
      - component: Registry
        operator: replace
        value: mirror.id.internal
```

### Why this is right

- `apiVersion: apps/v1` — generator never emits capitalised groups; the schema doc
  in [`knowledge/10-propagation-schema.md`](../../knowledge/10-propagation-schema.md)
  is the source of truth.
- No `replicaScheduling` at all — the user did not ask for replica distribution, so
  the generator does not invent one.
- `propagateDeps: true` — the skill defaults this to `true` for any Deployment because
  Deployments routinely reference ConfigMaps/Secrets.
- `imageOverrider` with `component: Registry` instead of a plaintext path — for the
  six known workload kinds (Pod, Deployment, ReplicaSet, DaemonSet, StatefulSet, Job)
  the structured override is type-safe and cannot mis-target the image field.

## Side-by-side at-a-glance

| Aspect                          | Without skill         | With skill            |
|---------------------------------|-----------------------|-----------------------|
| Lines of YAML                   | 47 (with one fluffy spurious replicaScheduling block) | 26 |
| Hard-rule errors                | 3                     | 0                     |
| Deprecation warnings            | 1                     | 0                     |
| Will admit to the apiserver     | ❌ (R3 webhook reject) | ✓                     |
| Override actually applies       | ❌ (silent no-op, R5)  | ✓                     |
| ConfigMap reaches member clusters | ❌ (no propagateDeps) | ✓                     |
| Result on `kubectl apply`       | Error + crashloop     | Working               |

## How to replay

```
# Regenerate the with-skill output from the intent:
python3 ../../scripts/generate_policy.py \
    --intent intent.json \
    --out /tmp/with-skill.regen.yaml
diff with-skill.yaml /tmp/with-skill.regen.yaml   # empty

# Re-audit the without-skill output:
python3 ../../scripts/audit_policy.py without-skill.yaml --format text
```

## Why this matters

The five bugs above are not pathological. They are the most common LLM mistakes on
Karmada policies, drawn directly from the failure cases the upstream proposal author
listed in [karmada-io/community#190](https://github.com/karmada-io/community/issues/190).
Every operator who has tried "just ask Cursor to make me a Karmada policy" has hit at
least three of them.

The split that makes the skill work: **structure is mechanical (the generator) and
intent is semantic (the LLM)**. The LLM produces the JSON intent. The generator
produces the YAML. The auditor verifies before the user applies. None of the three
steps requires the LLM to remember API field names — which is the part LLMs are
worst at.
