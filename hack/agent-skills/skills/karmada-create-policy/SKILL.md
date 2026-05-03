---
name: karmada-create-policy
description: Generate valid PropagationPolicy / ClusterPropagationPolicy / OverridePolicy / ClusterOverridePolicy YAML using the deterministic policy-generator script, never by free-form drafting.
when_to_load: |
  Trigger when the user asks to create, write, or scaffold a Karmada policy,
  propagate a workload, distribute across clusters, override a field per
  cluster, or any phrasing that resolves to "produce a Karmada policy YAML".
  Always loads `karmada-knowledge` first.
---

# karmada-create-policy

This skill exists for one reason: **agents that free-hand Karmada policy YAML
get fields wrong**. The skill forces the agent through a deterministic
generator so the YAML is schema-valid by construction.

## Hard rules

1. **Do not draft policy YAML token-by-token.** Always invoke
   `scripts/policy-generator.py` or copy from `templates/`.
2. **Confirm intent before generating.** If `replicaSchedulingType` or
   target clusters are ambiguous, ask one clarifying question first.
3. **One policy = one workload class.** If the user wants to propagate
   three different kinds, produce three policies (or one policy with
   three `resourceSelectors`, never three policies fused into one
   `placement`).

## Workflow

### Step 1 — gather inputs

Required for any policy:

- `kind`: one of `PropagationPolicy`, `ClusterPropagationPolicy`,
  `OverridePolicy`, `ClusterOverridePolicy`
- `name` (and `namespace` for namespaced kinds)
- For propagation: `target` `apiVersion`/`kind`/`name` and at least one of
  `clusters`, `clusterAffinityLabelSelector`, or `spreadByField`
- For override: `target` selector and the override rule(s)

### Step 2 — pick the right shape

| User says…                                     | Use                                  |
|------------------------------------------------|--------------------------------------|
| "deploy nginx to clusters A and B"             | `PropagationPolicy` + cluster names  |
| "spread across all EU regions"                 | `PropagationPolicy` + `spreadByField: region` |
| "fail over from primary to backup cluster"     | `PropagationPolicy` + `clusterAffinities` (plural, ordered) |
| "use a different image registry in China"      | `OverridePolicy` with `imageOverrider` |
| "global rule, no namespace"                    | `Cluster*Policy` variant             |

### Step 3 — invoke the generator

```bash
python3 hack/agent-skills/scripts/policy-generator.py \
  --kind PropagationPolicy \
  --name <name> \
  --namespace <ns> \
  --target-api-version apps/v1 \
  --target-kind Deployment \
  --target-name <workload> \
  --clusters <comma-separated> \
  [--replica-scheduling Duplicated|Divided] \
  [--spread-by-field region|zone|cluster|provider] \
  [--output policy.yaml]
```

The generator validates inputs, refuses unknown kinds, and emits canonical
YAML. If it errors, **report the error verbatim** to the user and ask them
to refine — do not "fix it up" in your head and produce YAML anyway.

### Step 4 — verify

After writing the file, run:

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG apply --dry-run=server -f <file>
```

If the user has no live Karmada control plane, fall back to `kubectl
apply --dry-run=client -f` and explicitly note that server-side validation
was skipped.

## Templates

- [`templates/propagation-policy.yaml`](../../templates/propagation-policy.yaml)
- [`templates/override-policy.yaml`](../../templates/override-policy.yaml)

## Worked example

[`examples/multi-country-setup/`](../../examples/multi-country-setup/) — a
deployment propagated to EU + US members, with a per-region image override.

## Failure modes the agent must catch

- User asks for `replicas: 3` "on every cluster" — that is `Duplicated`,
  not `Divided`. Confirm before generating.
- User lists clusters that don't exist — the generator does not check this
  (no live API access). Mention it explicitly: *"I'm not validating that
  these cluster names exist; verify with `kubectl get clusters`."*
- User mixes `clusterAffinity` (singular) and `clusterAffinities` (plural).
  Pick one — they are not additive.
