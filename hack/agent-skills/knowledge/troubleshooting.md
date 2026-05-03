# Troubleshooting Playbooks

Symptom-indexed entries. Each entry: **observable** → **likely cause**
→ **command to confirm** → **fix**. Used by `karmada-debug-propagation`.

## T1 — "I applied a Deployment but it never appears on member clusters"

**Observable:** workload exists in host plane, no pods in member.

**Likely cause (in order of frequency):**

1. No PropagationPolicy matches the workload.
2. PropagationPolicy matches but `placement.clusterAffinity` selects
   no clusters.
3. ResourceBinding created but `Scheduled=False`.
4. Work applied but member-cluster apiserver rejected it (CRD missing,
   RBAC, etc.).

**Confirm:**
```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> get rb
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> describe rb <name>
```

**Fix:** depends on which condition failed — see playbooks T2–T5.

## T2 — "ResourceBinding exists but `Scheduled=False`"

**Observable:** `status.conditions[?type=="Scheduled"].message` says
`0/N clusters are available`.

**Likely cause:** the candidate set after applying `clusterAffinity`,
`clusterTolerations`, and `spreadConstraints` is empty.

**Confirm:**
```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG get clusters --show-labels
```

**Fix:** loosen the selector OR add the missing label to the cluster
(`kubectl --kubeconfig $KARMADA_KUBECONFIG label cluster <name> region=eu`).

## T3 — "Work has `Applied=False` with `already exists`"

**Observable:**
```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n karmada-es-<cluster> \
  describe work <name> | grep -A5 'Conditions:'
# Type: Applied, Status: False, Message: ... already exists ...
```

**Likely cause:** a resource with the same identity exists in the
member cluster and was not created by Karmada.

**Fix:** decide ownership.

- If Karmada should own it: set
  `spec.conflictResolution: Overwrite` on the PropagationPolicy.
- If the member-side resource is the source of truth: narrow the
  PropagationPolicy `resourceSelectors` so it doesn't match.

Never set `Overwrite` blindly — it will silently clobber prod state.

## T4 — "OverridePolicy seems to do nothing"

**Observable:** the override exists, the workload propagates, but the
target field is unchanged in the member cluster.

**Likely causes:**

1. `targetCluster` doesn't include the cluster you're checking.
2. `plaintext.path` is JSONPath instead of JSON Pointer (the field
   silently does not exist at that path, so the patch is a no-op on
   `add`/`replace` for `add`-only paths; some forms error, some don't).
3. `imageOverrider` was misspelled `imageOverride` and the apiserver
   rejected the policy (check `karmada-webhook` logs).

**Confirm:**
```bash
python3 hack/agent-skills/scripts/validate-policy.py override.yaml
```

If lint reports `KMD210` (`imageOverride` typo) or `KMD221` (JSONPath),
that is the cause.

## T5 — "Pods exist but the wrong number per cluster"

**Observable:** total replicas differ from `spec.replicas`, or the split
is unexpected.

**Likely cause:** confusion between `Duplicated` and `Divided`.

| Setting                          | `replicas: 3` means          |
|----------------------------------|------------------------------|
| `replicaSchedulingType: Duplicated` | 3 on each target cluster |
| `replicaSchedulingType: Divided`    | 3 total, split            |

**Fix:** set the type explicitly. Do not rely on defaults.

## T6 — "Member cluster shows `Ready=False`"

**Observable:**
```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG describe cluster <name> \
  | grep -A 10 Conditions
```

**Likely cause:**

- Pull mode: `karmada-agent` pod in the member cluster is down or
  cannot reach the host apiserver.
- Push mode: the kubeconfig `Secret` referenced by `Cluster.spec` is
  invalid or expired.

**Fix:**
- Pull: `kubectl --kubeconfig $MEMBER -n karmada-system logs deploy/karmada-agent`.
- Push: rotate the kubeconfig secret and update `Cluster.spec.secretRef`.

## T7 — "Policy was accepted yesterday, rejected today"

**Likely cause:** a Karmada upgrade tightened webhook validation. Lint
the existing policy:

```bash
python3 hack/agent-skills/scripts/validate-policy.py mypolicy.yaml
```

If the linter is clean and the webhook still rejects, capture the
webhook log:

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n karmada-system logs \
  deploy/karmada-webhook --tail=200
```
