# Example — Active / Passive Failover

A worked scenario showing **cluster-level** active/passive failover
plus **application-level** graceful drain.

## Scenario

- `member-prod-east` is the primary; everything runs there.
- `member-prod-west` is the warm backup; nothing runs there until the
  primary is unhealthy for ≥ 120 seconds.
- When failover happens, in-flight pods drain for 60s before the
  workload is purged from the failed cluster.

## Manifests

- [`deployment.yaml`](deployment.yaml) — the workload.
- [`propagation-policy.yaml`](propagation-policy.yaml) — uses
  `clusterAffinities` (plural, ordered) and the `failover.application`
  block.

## Reproduce with the generator

```bash
python3 hack/agent-skills/scripts/policy-generator.py \
  --kind PropagationPolicy \
  --name api-failover --namespace default \
  --target-kind Deployment --target-name api \
  --affinity-group primary=member-prod-east \
  --affinity-group backup=member-prod-west \
  --replica-scheduling Duplicated \
  --failover-toleration-seconds 120 \
  --failover-purge-mode Graciously \
  --failover-grace-period-seconds 60 \
  --output hack/agent-skills/examples/failover/propagation-policy.yaml
```

## Verify with the linter

```bash
python3 hack/agent-skills/scripts/validate-policy.py \
  hack/agent-skills/examples/failover/propagation-policy.yaml
# expected: ok: no findings
```

## Why `clusterAffinities` (plural), not `clusterAffinity` (singular)

The plural form is **ordered**. The scheduler tries the first affinity
group; only if it cannot place the workload there does it move to the
next. The singular form has no failover semantics — it is a single
candidate set.

A common AI-agent mistake is to fill *both* fields hoping for combined
behavior. The validator catches this as `KMD100`.

## What to watch during a failover drill

```bash
# Trigger by tainting the primary:
kubectl --kubeconfig $KARMADA_KUBECONFIG taint cluster member-prod-east \
  test=true:NoSchedule

# Watch the binding's active group migrate:
kubectl --kubeconfig $KARMADA_KUBECONFIG -n default get rb -o yaml \
  | grep -E 'schedulerObservedAffinityName|clusters:'
```

After ~120 seconds, `schedulerObservedAffinityName` should flip from
`primary` to `backup` and `spec.clusters` should rewrite to
`member-prod-west`.
