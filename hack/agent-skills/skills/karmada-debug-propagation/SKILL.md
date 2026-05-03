---
name: karmada-debug-propagation
description: Diagnose why a workload submitted to the Karmada control plane is not running on member clusters, by walking the PropagationPolicy → ResourceBinding → Work → member-cluster chain in a fixed order.
when_to_load: |
  Trigger when the user reports any of: "my Deployment isn't showing up on
  member clusters", "PropagationPolicy doesn't seem to apply", "ResourceBinding
  is empty", "Work is stuck", "no scheduling decision", "scheduler can't find
  cluster". Always loads `karmada-knowledge` first.
---

# karmada-debug-propagation

Karmada propagation has **four hand-offs**. A failure at any one of them
looks the same from a user's seat — *"my workload isn't there"* — so the
agent must walk the chain in order, not jump to a hypothesis.

```
1. PropagationPolicy   selects the workload
2. ResourceBinding     records the scheduling decision
3. Work                materializes the manifest per cluster
4. Member cluster      actually runs the resource
```

Skipping ahead is the single biggest mistake AI agents make here.

## The 7-step diagnostic

### Step 1 — confirm the workload exists in the host control plane

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> get <kind> <name>
```

If this is missing, the user hasn't applied it to the **Karmada apiserver**
(common confusion with the host-cluster apiserver). Stop and clarify.

### Step 2 — find a matching PropagationPolicy

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> get pp,cpp -A
```

Verify a policy's `resourceSelectors[*]` actually matches the workload's
`apiVersion`/`kind`/`name`. Common mismatches:

| Symptom in selector             | Reality                          |
|---------------------------------|----------------------------------|
| `apiVersion: v1`, `kind: Deployment` | Deployment is `apps/v1`     |
| `name: my-app`                  | workload is `name: my-app-v2`    |
| no `name`, no `labelSelector`   | matches *every* resource of that kind in ns |

If multiple policies match, the higher `spec.priority` wins. Ties resolve
to the lexicographically smaller name.

### Step 3 — find the ResourceBinding (RB / CRB)

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n <ns> get rb -o wide
kubectl --kubeconfig $KARMADA_KUBECONFIG describe rb <rb-name> -n <ns>
```

The RB name is `<workload-name>-<kind-lowercased>`. Look at:

- `.spec.clusters` — the scheduler's chosen target list.
- `.status.conditions` — `Scheduled=True` is what you want.

Failure modes:

- **RB does not exist** → `karmada-controller-manager` did not see the
  workload. Check its logs (Step 7).
- **RB exists, `.spec.clusters` empty, `Scheduled=False`** → scheduler
  could not satisfy placement. Check the condition `message`.

### Step 4 — read the scheduling decision message

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG describe rb <rb-name> -n <ns> \
  | grep -A 20 'Conditions:'
```

Common messages and what they mean:

| Message fragment                      | Cause                                     |
|---------------------------------------|-------------------------------------------|
| `0/N clusters are available`          | no cluster matches `clusterAffinity`      |
| `cluster(s) were not ready`           | targets exist but `Ready=False`           |
| `cluster(s) had taint that the pod didn't tolerate` | missing `clusterTolerations` |
| `insufficient resource`               | replicas exceed cluster capacity          |

### Step 5 — find the Work objects (one per target cluster)

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG -n karmada-es-<cluster> get work
kubectl --kubeconfig $KARMADA_KUBECONFIG -n karmada-es-<cluster> describe work <work-name>
```

The execution namespace is **always** `karmada-es-<cluster-name>`. Look at
`.status.conditions` — `Applied=True` and `Available=True` are the goal.

Failure modes:

- `Applied=False` with `message: ... already exists` → conflict with an
  existing resource in the member cluster; either it predates Karmada or
  another policy owns it. Set `conflictResolution: Overwrite` only if you
  intend to take ownership.
- `Applied=True` but no resource in the member cluster → cluster agent
  not syncing; check `karmada-agent` (pull-mode) or member access.

### Step 6 — verify on the member cluster directly

```bash
kubectl --kubeconfig $MEMBER_KUBECONFIG -n <ns> get <kind> <name>
```

If the resource exists but with unexpected fields, an OverridePolicy is
mutating it — see `karmada-explain-placement` for which override fired.

### Step 7 — controller-manager logs (last resort)

```bash
kubectl --kubeconfig $HOST_KUBECONFIG -n karmada-system logs \
  deploy/karmada-controller-manager --tail=200 | grep -i <workload-name>
```

Then `karmada-scheduler` for placement issues, `karmada-agent` (on the
member) for sync issues.

## What the agent must NOT do

- **Don't recommend `kubectl delete` to "force a re-sync".** Karmada
  reconciles on its own; deleting RBs or Works often masks the real cause.
- **Don't suggest editing the Work directly.** It is reconstructed from
  the binding; edits will revert.
- **Don't assume the policy is wrong because the workload is missing.**
  Two of the seven steps live downstream of the policy.

## References

- Object chain documented in `knowledge/components.md`.
- Common failure modes catalogued in `knowledge/troubleshooting.md`.
