# Example 03 — A real-looking propagation-failure bundle

This bundle reproduces the *T-NoSchedule* scenario from `knowledge/02-troubleshooting.md`:
the user has a Service in the Karmada control plane, the PropagationPolicy is admitted,
the ResourceBinding is created — but the scheduler refuses to schedule because the
label selector matches no Cluster.

## Replay

```
python3 ../../scripts/debug_propagation.py \
    --bundle bundle.yaml \
    --target v1/Service/default/checkout
```

Expected verdict (verbatim):

```json
{
  "verdict": "schedule-failed",
  "hop_reached": "ResourceBinding",
  "evidence": [
    "found target object Service/checkout",
    "1 policy(ies) match: checkout-pp",
    "1 binding(s) present",
    "RB.status[Scheduled]=False reason='NoClusterFit' message='no cluster matches affinity environment=production'"
  ],
  "next_steps": [
    "confirm Cluster objects have the labels the placement expects",
    "if clusterAffinities (plural), the first group with any match short-circuits — list preferred groups first",
    "see knowledge/02-troubleshooting.md case T-NoSchedule"
  ]
}
```

The fix is to label the Cluster objects: `kubectl label cluster member1 environment=production`.
