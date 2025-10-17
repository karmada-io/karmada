---
title: Multi-Resource Dependency Distribution Conflict Resolution
authors:
  - "@Kexin2000"
reviewers:
  - ""
approvers:
  - "@"
creation-date: 2025-09-30
---

# Multi-Resource Dependency Distribution Conflict Resolution

## Summary

This proposal enhances Karmada’s dependency distribution in multi-parent scenarios. When the same dependent resource (for example ConfigMap/Secret) is referenced by multiple workloads, the system aggregates governance policies deterministically, maintains a single attached ResourceBinding, and tracks all parents via the RequiredBy field. The change improves predictability, observability, and operability without breaking existing usage patterns.

## Motivation

As applications scale, dependent resources are frequently shared across workloads. Without clear rules, strategy divergence can cause conflicts and unpredictable behavior. We provide unified aggregation rules and better telemetry to keep behavior deterministic and debuggable.

## Goals

- Define deterministic aggregation rules for multi-parent dependency distribution.
- Keep a single attached ResourceBinding per shared dependency with accurate RequiredBy tracking.
- Improve observability via events, metrics, and structured logs.
- Provide E2E coverage and documentation to guide users and operators.

## Non-Goals

- Redesign scheduling algorithms or deprecate existing policy-based distribution.
- Introduce governance-breaking changes; existing semantics remain compatible.

## Proposal

### Aggregation Rules (selected approach)

When a dependent resource is referenced by multiple parents, aggregate policy fields as follows:

- ConflictResolution: if any parent specifies Overwrite, result is Overwrite; otherwise Abort.
- PreserveResourcesOnDeletion: logical OR across parents (true if any parent sets true).

Additional behaviors:

- RequiredBy tracking: a single attached ResourceBinding is created/maintained for the shared dependency; RequiredBy lists all parent bindings and is updated when parents change.
- Re-aggregation triggers: parent set changes; parent RB/CRB governance fields change; workload spec changes that alter dependency graphs.
- Debounce updates: only patch the attached ResourceBinding when the aggregated result actually changes.

### Observability

- Events: DependencyPolicyConflict (conflict detected), DependencyPolicyAggregated (aggregation succeeded).
- Metrics: dependency_policy_conflicts_total (count conflicts), dependency_parents_count (number of parents during aggregation).
- Logs: detailed logs with prefix [dep-agg] for easy tracing.

### API & Compatibility

- Reuses existing PropagateDeps and RequiredBy in ResourceBinding; no breaking API change.
- Fully backward compatible; opt-in behavior follows existing feature-gate usage where applicable.

## Test Plan

- New E2E test verifies multi-parent tracking: a single attached ResourceBinding is produced; RequiredBy contains all parents; RequiredBy updates when a parent stops referencing the dependency. Tests focus on non-governance assertions of dependency distribution.

## Alternatives Considered

- Forbid shared dependencies (rejected: poor compatibility and usability).
- Order-based inheritance (rejected: low observability, timing-dependent behavior).
- Always use defaults on conflict (rejected: ignores user intent, low transparency).

## Implementation History

- PR1: Events for conflict and aggregation.
- PR2: Core aggregation logic, re-aggregation triggers, debounce updates.
- PR3: Metrics and [dep-agg] logs.
- PR4: User guide and changelog updates.
- PR5: E2E tests for multi-parent dependency tracking.

