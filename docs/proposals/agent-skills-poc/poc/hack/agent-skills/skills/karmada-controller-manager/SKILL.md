---
name: karmada-controller-manager
description: Reason about karmada-controller-manager behavior. Use when the user is debugging which controller owns a behavior, reading controller logs, or wants to know which sub-controller handles a CRD.
metadata:
  type: component
  loads:
    - knowledge/03-components.md
    - knowledge/00-overview.md
  status: scaffold
---

# karmada-controller-manager

> **Status:** scaffold. Full troubleshooting tree and log-pattern catalog land in
> mentorship weeks 9–10.

The karmada-controller-manager binary hosts a fleet of controllers (see
`pkg/controllers/` in the karmada repo). This skill lets an agent answer "which
controller is responsible for X?" without searching the codebase from scratch.

## Controllers and their CRDs (one line each)

| Controller                | Watches                                | Writes                                   |
|---------------------------|-----------------------------------------|------------------------------------------|
| binding                   | ResourceTemplate + PropagationPolicy    | ResourceBinding                          |
| execution                 | Work                                    | applies manifest to member cluster       |
| cluster                   | Cluster                                 | execution-space namespaces, health       |
| namespace                 | Namespace (control-plane)               | propagates namespaces to members         |
| taint                     | Cluster taints                          | binding evictions                        |
| graceful-eviction         | RB.spec.gracefulEvictionTasks           | orchestrated migration                   |
| mcs                       | MultiClusterService                     | service exports + EndpointSlices         |
| applicationfailover       | RB health on members                    | reschedule decisions                     |
| federatedhpa              | FederatedHPA                            | per-cluster HPA shards                   |
| cronfederatedhpa          | CronFederatedHPA                        | cron-driven scale                        |
| workloadrebalancer        | WorkloadRebalancer                      | rebalance triggers                       |
| hpascaletargetmarker      | HPA-owned workloads                     | scale-target labels                      |

## Planned content

- A `knowledge/06-controller-log-patterns.md` cookbook mapping common log lines to
  controllers ("failed to sync work" → execution controller).
- A `scripts/diagnose_controller.py` that takes a controller-manager log fragment
  and points at the offending sub-controller + the source file in `pkg/controllers/`.
