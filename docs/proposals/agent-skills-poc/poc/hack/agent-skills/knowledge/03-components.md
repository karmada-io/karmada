# Karmada Components — what each one does and when an agent should mention it

The control plane is a small handful of binaries. When a user reports a symptom, the
agent should be able to name the component most likely responsible for the failure.

## karmada-apiserver
- **Binary:** `cmd/karmada-apiserver/` (forked stripped kube-apiserver build).
- **Talks to:** Karmada etcd.
- **Symptoms that point here:** "I can't even apply the policy", "CRDs missing",
  TLS errors, webhook timeouts on admit.

## karmada-controller-manager
- **Binary:** `cmd/controller-manager/`.
- **Controllers it runs:** binding, execution, cluster, namespace, taint, mcs,
  graceful-eviction, federatedhpa, cronfederatedhpa, multiclusterservice,
  applicationfailover, workloadrebalancer, hpascaletargetmarker, descheduler glue.
  See `pkg/controllers/`.
- **Symptoms that point here:** "RB never appeared", "RB exists but Work never appeared",
  "policy is stuck Terminating".

## karmada-scheduler
- **Binary:** `cmd/scheduler/`.
- **Job:** evaluate `spec.placement` on every ResourceBinding and write
  `status.targetClusters` (or `spec.clusters` in newer schemas) with the picked clusters
  and divided replica counts.
- **Symptoms that point here:** `RB.status.conditions[Scheduled].status=False`.

## karmada-webhook
- **Binary:** `cmd/webhook/`.
- **Job:** ValidatingWebhookConfiguration + MutatingWebhookConfiguration for the policy
  CRDs. Sets defaults, rejects mutually exclusive fields, enforces minimum items.
- **Symptoms that point here:** "the apply is rejected with a webhook error",
  "field X cannot be changed".

## karmada-aggregated-apiserver
- **Binary:** `cmd/aggregated-apiserver/`.
- **Job:** proxy `clusters/<name>/proxy/<api>` requests through to member apiservers.
- **Symptoms that point here:** "`kubectl --cluster=member1 get pods` returns 403 or
  hangs"; the proxy or the member's kubeconfig is at fault.

## karmada-search
- **Binary:** `cmd/karmada-search/`.
- **Job:** separate aggregated apiserver that indexes member resources via
  `ResourceRegistry` CRD and offers `search.karmada.io/v1alpha1` for cross-cluster reads.
  Independent of propagation — turning it off does not break propagation.
- **Symptoms that point here:** "I can't search pods across clusters", "the search
  endpoint times out".

## karmada-descheduler
- **Binary:** `cmd/descheduler/`.
- **Job:** periodically re-evaluate placements with stale capacity assumptions and
  trigger reschedule.
- **Symptoms that point here:** "replicas never rebalance after a member cluster scales
  up/down".

## karmada-scheduler-estimator
- **Binary:** `cmd/scheduler-estimator/`. One instance *per member cluster*.
- **Job:** report unschedulable pod count for the scheduler's dynamic-weight strategy.
- **Symptoms that point here:** "dynamic weight always behaves as static / always picks
  the first cluster". Estimator likely dead or unreachable.

## karmada-metrics-adapter
- **Binary:** `cmd/metrics-adapter/`.
- **Job:** federated metrics for FederatedHPA. Aggregates pod metrics across clusters
  so the multi-cluster HPA can decide.
- **Symptoms that point here:** "FederatedHPA never scales", "the HPA's current value
  is `<unknown>`".

## karmada-operator (out-of-tree, lives in `operator/`)
- **Job:** lifecycle-manage Karmada control planes themselves via the `Karmada` CRD.
  Useful for users running Karmada-as-a-Service. Most users do not run this.

## A simple decision tree for "where do I look?"

```
"can't apply a policy"            → karmada-apiserver, karmada-webhook
"policy applied, no RB"           → karmada-controller-manager (binding controller)
"RB exists, Scheduled=False"      → karmada-scheduler (and the Cluster labels)
"RB Scheduled=True, no Work"      → karmada-controller-manager (binding→work step)
"Work exists, member is empty"    → karmada-controller-manager (execution controller),
                                     check member-side admission/RBAC
"override didn't apply"           → karmada-webhook (validation passed but path wrong)
                                     or karmada-controller-manager (override interpreter)
"can't search cross-cluster"      → karmada-search
"HPA never scales"                → karmada-metrics-adapter, karmada-controller-manager
```

When a user pastes a stack trace, the agent should grep it for one of the binary names
above before guessing.
