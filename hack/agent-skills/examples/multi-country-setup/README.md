# Example — Multi-Country Setup

A worked end-to-end scenario the `karmada-create-policy` skill points
agents to when a user asks for a "global rollout with regional tweaks".

## Scenario

Deploy `nginx` to two member clusters:

| Cluster      | Region | Image registry needed                               |
|--------------|--------|-----------------------------------------------------|
| `member-eu`  | eu     | upstream `nginx`                                    |
| `member-cn`  | cn     | mirrored `registry.cn-hangzhou.aliyuncs.com/myorg`  |

We want **3 replicas total**, divided across the two clusters by
available capacity. The CN cluster must use the mirrored registry.

## Manifests

- [`deployment.yaml`](deployment.yaml) — the workload, applied to the
  Karmada apiserver.
- [`propagation-policy.yaml`](propagation-policy.yaml) — picks both
  clusters, divides replicas, and enables dependency propagation.
- [`override-policy.yaml`](override-policy.yaml) — swaps the image
  registry only on `member-cn`.

## Reproduce with the generator

```bash
cd hack/agent-skills

python3 scripts/policy-generator.py \
  --kind PropagationPolicy \
  --name nginx-global --namespace default \
  --target-kind Deployment --target-name nginx \
  --clusters member-eu,member-cn \
  --replica-scheduling Divided \
  --propagate-deps \
  --output examples/multi-country-setup/propagation-policy.yaml

python3 scripts/policy-generator.py \
  --kind OverridePolicy \
  --name nginx-cn-mirror --namespace default \
  --target-kind Deployment --target-name nginx \
  --clusters member-cn \
  --override-image-registry registry.cn-hangzhou.aliyuncs.com/myorg \
  --output examples/multi-country-setup/override-policy.yaml
```

## Apply

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG apply -f deployment.yaml
kubectl --kubeconfig $KARMADA_KUBECONFIG apply -f propagation-policy.yaml
kubectl --kubeconfig $KARMADA_KUBECONFIG apply -f override-policy.yaml
```

## What to look for

After a few seconds:

```bash
kubectl --kubeconfig $KARMADA_KUBECONFIG get rb -n default
# Expect a ResourceBinding for nginx with Scheduled=True and
# clusters [member-eu, member-cn].

kubectl --kubeconfig $MEMBER_CN_KUBECONFIG get deploy nginx -o yaml \
  | grep image:
# Expect: registry.cn-hangzhou.aliyuncs.com/myorg/nginx:...
```
