---
title: Hierarchical Spread Constraints

reviewers:
- "@RainbowMango"
- "@XiShanYongYe-Chang"
- "@seanlaii"
- "@zhzhuang-zju"

approvers:
- "@RainbowMango"

creation-date: 2026-04-28

---

# Hierarchical Spread Constraints

## Summary

Karmada's `SpreadConstraint.SpreadByField` API declares four topology fields:
`provider`, `region`, `zone`, and `cluster`. These fields naturally form a
fixed hierarchy:

```text
provider -> region -> zone -> cluster
```

However, the current scheduler only supports a subset of the intended behavior,
notably `cluster` and `region + cluster`. This proposal defines a complete
hierarchical selection model for all ordered `SpreadByField` combinations, such
as `zone + cluster`, `provider + cluster`, `region + zone + cluster`, and
`provider + region + zone + cluster`, without introducing new API fields.

The scheduler should interpret spread constraints as constraints on the declared
levels of the hierarchy. Any topology level that is not declared by the user is
not treated as an implicit spreading requirement. For example,
`provider + zone + cluster` spreads zones within selected providers, but does
not require workloads to spread across regions.

## Motivation

Users rely on spread constraints to improve availability, reduce correlated
failure risk, and control placement across multi-cloud and multi-region
environments. The API already advertises support for `provider`, `region`,
`zone`, and `cluster`, but scheduler selection is incomplete. As a result, users
can declare combinations that pass API validation but are not handled by the
selection algorithm.

Completing the behavior with one hierarchical model makes the feature easier to
understand and maintain. It also avoids repeatedly adding one-off branches for
every new combination of `SpreadByField` values.

### Goals

- Define a complete scheduling semantic for `provider`, `region`, `zone`, and
  `cluster` spread constraints.
- Support all valid ordered combinations that include `cluster`, including:
  - `cluster`
  - `zone + cluster`
  - `region + cluster`
  - `region + zone + cluster`
  - `provider + cluster`
  - `provider + region + cluster`
  - `provider + zone + cluster`
  - `provider + region + zone + cluster`
- Preserve the current requirement that `cluster` must be included when
  `SpreadByField` is used.
- Preserve the behavior of existing `cluster` and `region + cluster`
  configurations as much as possible.
- Define failure handling, scoring, tie-breaking, and test strategy for the
  complete feature.

### Non-Goals

- Introduce new fields to `PropagationPolicy`, `ClusterPropagationPolicy`,
  `ResourceBinding`, or `ClusterResourceBinding`.
- Change the meaning of `SpreadByLabel`.
- Add custom user-defined topology fields.
- Change cluster filtering behavior for missing provider, region, or zone
  properties.
- Define preemption behavior for spread constraints.

## Proposal

This proposal introduces a generic hierarchical spread constraint selection
model. The scheduler will build a topology tree from feasible clusters and apply
declared constraints from the highest declared level down to `cluster`.

The hierarchy is fixed:

```text
provider -> region -> zone -> cluster
```

The user's declared `SpreadByField` values determine which tree levels are
constrained. Skipped levels are not selected, do not become part of group
identity, and do not create additional spreading requirements.

### User Stories

#### Story 1: Spread replicas across zones for availability

As an application owner, I want replicas of a service to be placed in different
failure zones so that a zone outage does not take down all replicas. I do not
care which provider or region the clusters belong to because my candidate
clusters have already been filtered by `clusterAffinity`.

For this case, I configure `zone + cluster`. The scheduler selects enough zone
groups first, and then selects the best clusters from those zones.

#### Story 2: Spread workloads across regions and zones

As a platform administrator, I manage clusters in multiple regions. I want a
business-critical workload to span regions, and within the selected regions I
also want it to avoid concentrating all replicas in the same zone.

For this case, I configure `region + zone + cluster`. The scheduler first
selects regions, then selects zone groups under the selected regions. A zone is
identified by its declared ancestors, so `east/zone-a` and `west/zone-a` are
different groups.

#### Story 3: Spread workloads across providers while skipping regions

As a cost and availability owner, I want a workload to use more than one cloud
provider and to spread across failure zones inside those providers. I do not
want to require region spreading because the available regions differ between
providers and the region names are not important for this workload.

For this case, I configure `provider + zone + cluster`. The scheduler selects
providers, then zone groups within the selected providers. Because `region` is
not declared, region is ignored for group identity and selection.

#### Story 4: Use the full provider, region, zone, and cluster hierarchy

As an operator of a large fleet, I need strict placement across cloud providers,
regions, zones, and clusters for a highly available workload. I want the
scheduler to enforce each declared level in a predictable top-down order.

For this case, I configure `provider + region + zone + cluster`. The scheduler
uses the full hierarchy and selects groups at every level before final cluster
selection.

### Terminology

- **Topology level**: One of `provider`, `region`, `zone`, or `cluster`.
- **Declared level**: A topology level that appears in
  `.spec.placement.spreadConstraints[*].spreadByField`.
- **Ancestor path**: The ordered topology values above a level. For example, the
  ancestor path for a zone may be `provider=P1, region=R1`.
- **Group identity**: A topology group key formed from the declared level and the
  relevant declared ancestor path.
- **Topology tree**: The hierarchical view of feasible clusters grouped by
  provider, region, zone, and cluster.

### Supported SpreadByField Combinations

The scheduler should accept every combination that follows the fixed hierarchy
and includes `cluster`.

| Combination | Selection behavior |
| --- | --- |
| `cluster` | Select clusters directly. |
| `zone + cluster` | Select zones globally, then select clusters from selected zones. |
| `region + cluster` | Select regions, then select clusters from selected regions. |
| `region + zone + cluster` | Select regions, then select zones within selected regions, then select clusters. |
| `provider + cluster` | Select providers, then select clusters from selected providers. |
| `provider + region + cluster` | Select providers, then regions within selected providers, then select clusters. |
| `provider + zone + cluster` | Select providers, then zones within selected providers, then select clusters. Regions are not considered. |
| `provider + region + zone + cluster` | Select providers, then regions, then zones, then clusters. |

If a user skips a middle level, the skipped level must not become an implicit
spreading requirement. For example, `provider + zone + cluster` constrains
providers and zones, but not regions.

### Hierarchical Semantics

Group identity must include only the declared ancestor levels. Skipped ancestor
levels are ignored for identity and selection. This preserves the meaning of the
user-declared hierarchy while avoiding implicit constraints.

Examples:

- `zone + cluster`: a zone group is keyed by `zone`.
- `region + zone + cluster`: a zone group is keyed by `region/zone`.
- `provider + zone + cluster`: a zone group is keyed by `provider/zone`.
- `provider + region + zone + cluster`: a zone group is keyed by
  `provider/region/zone`.

This means zones with the same zone name in different declared ancestors are
treated as different groups. For example, in `region + zone + cluster`,
`R1/Z1` and `R2/Z1` are different zone groups. In `provider + zone + cluster`,
`P1/Z1` and `P2/Z1` are different zone groups. If `region` is not declared, the
scheduler does not require or prefer region spreading.

### Selection Flow Overview

![spreadconstraints selection flow](spreadconstraints-selection-flow.drawio.svg)

The scheduler selection flow has these conceptual stages:

1. Normalize constraints into the fixed topology order.
2. Build a topology tree from feasible clusters.
3. Traverse declared levels from the highest declared level to `cluster`.
4. Skip undeclared levels without applying group identity or spreading
   requirements.
5. Select the best groups at each declared level according to `minGroups`,
   `maxGroups`, score, and capacity.
6. Finalize the selected clusters with existing cluster score, overflow order,
   available resource, and deterministic name tie-breaking.

The flowchart also shows the failure paths. If the constraint set is invalid,
the scheduler rejects it before building the topology tree. If a declared group
level cannot satisfy its `minGroups` or cannot provide enough downstream
clusters, scheduling fails. If final cluster de-duplication or resource-aware
selection cannot satisfy `cluster.minGroups`, scheduling also fails.

## Design Details

### Constraint Normalization

The scheduler should convert the list of spread constraints into a map keyed by
`SpreadByField`, then derive an ordered declared-level list using the fixed
hierarchy:

```text
provider -> region -> zone -> cluster
```

For example, the user may provide constraints in any YAML order, but
`provider + zone + cluster` is always evaluated as provider first, zone second,
and cluster last.

Validation already requires `cluster` to be present when any `SpreadByField` is
used. This proposal keeps that rule. Validation should also continue to reject
duplicate constraints for the same `SpreadByField`.

### Topology Tree Builder

The scheduler should build a tree from feasible clusters after filtering. Each
cluster contributes to the tree according to its topology properties:

```text
root
  provider
    region
      zone
        cluster
```

The tree stores aggregate information for every group:

- group name
- topology level
- ancestor path
- child groups
- clusters under the group
- score
- available replicas

This structure allows the selection algorithm to apply the same logic at every
declared level, instead of maintaining separate selector implementations for
regions, zones, and providers.

### Selection Flow

The selector evaluates declared levels from top to bottom.

At each non-cluster level, it selects child groups that satisfy that level's
`minGroups` and `maxGroups`. The selected groups define the candidate scope for
the next declared level. When a declared level is skipped, the selector does not
select groups at that level and does not enforce any `minGroups` or `maxGroups`
for it.

At the `cluster` level, the selector chooses final clusters from the remaining
candidate scope according to the cluster constraint.

The selection should fail if:

- there are fewer feasible groups than `minGroups` for a declared non-cluster
  level;
- no group combination can satisfy the downstream `cluster.minGroups`;
- the final cluster selection cannot satisfy `cluster.minGroups`;
- available resource requirements cannot be satisfied when replica scheduling
  requires resource-aware selection.

### Selection Algorithm Principles

The new algorithm is a top-down constrained selection over a topology tree. It
uses the same idea at every non-cluster level:

1. Work only inside the current candidate scope.
2. Build candidate groups for the next declared level.
3. Score each group using the clusters under that group.
4. Choose a group combination that satisfies that level's `minGroups` and
   `maxGroups`.
5. Keep only the clusters under the selected groups and continue to the next
   declared level.

This design is intentionally different from one selector per combination. The
selection mechanics for provider, region, and zone are the same; only the group
identity changes based on declared ancestors.

The algorithm follows these rules:

- **Declared levels only**: undeclared levels do not participate in selection.
- **Stable hierarchy order**: constraints are always evaluated as
  `provider -> region -> zone -> cluster`, regardless of YAML order.
- **Ancestor-aware identity**: group keys include declared ancestors. For
  example, a zone group may be `zone-a`, `region-a/zone-a`,
  `provider-a/zone-a`, or `provider-a/region-a/zone-a`.
- **Downstream feasibility**: a group combination is useful only if it can
  provide enough unique clusters for `cluster.minGroups`.
- **Deterministic result**: ties are broken by existing cluster ordering rules
  and stable group keys.
- **No duplicate final clusters**: a multi-zone cluster may help multiple zone
  groups become feasible, but the final result contains each cluster once.

### Pseudocode

The following pseudocode is intentionally close to Go, but it omits
implementation details that are not required for the proposal.

```go
var hierarchy = []SpreadFieldValue{
    SpreadByFieldProvider,
    SpreadByFieldRegion,
    SpreadByFieldZone,
    SpreadByFieldCluster,
}

func SelectBestClusters(placement Placement, clusters []ClusterDetailInfo) ([]ClusterDetailInfo, error) {
    constraints, declaredLevels, err := normalizeSpreadConstraints(placement.SpreadConstraints)
    if err != nil {
        return nil, err
    }
    if len(declaredLevels) == 0 {
        return selectClustersDirectly(clusters, placement)
    }

    tree := buildTopologyTree(clusters)
    scope := newSelectionScope(tree.Root)

    for _, level := range declaredLevels {
        constraint := constraints[level]
        if level == SpreadByFieldCluster {
            return finalizeClusters(scope.Clusters(), constraint, placement)
        }

        groups := scope.GroupsForDeclaredLevel(level, declaredLevels)
        selectedGroups, err := selectGroupsAtLevel(groups, constraint, constraints[SpreadByFieldCluster])
        if err != nil {
            return nil, err
        }
        scope = scope.NarrowTo(selectedGroups)
    }

    return nil, fmt.Errorf("cluster spread constraint is required")
}
```

Constraint normalization keeps validation and ordering separate from selection:

```go
func normalizeSpreadConstraints(spreadConstraints []SpreadConstraint) (
    map[SpreadFieldValue]SpreadConstraint,
    []SpreadFieldValue,
    error,
) {
    constraintMap := map[SpreadFieldValue]SpreadConstraint{}
    for _, constraint := range spreadConstraints {
        field := constraint.SpreadByField
        if field == "" {
            continue
        }
        if _, ok := constraintMap[field]; ok {
            return nil, nil, fmt.Errorf("multiple %s spread constraints are not allowed", field)
        }
        constraintMap[field] = constraint
    }

    if len(constraintMap) == 0 {
        return constraintMap, nil, nil
    }
    if _, ok := constraintMap[SpreadByFieldCluster]; !ok {
        return nil, nil, fmt.Errorf("cluster spread constraint is required")
    }

    declaredLevels := make([]SpreadFieldValue, 0, len(constraintMap))
    for _, level := range hierarchy {
        if _, ok := constraintMap[level]; ok {
            declaredLevels = append(declaredLevels, level)
        }
    }
    return constraintMap, declaredLevels, nil
}
```

Group identity uses declared ancestors only:

```go
func groupKeyFor(level SpreadFieldValue, cluster ClusterDetailInfo, declaredLevels []SpreadFieldValue) string {
    parts := []string{}
    for _, declared := range declaredLevels {
        if declared == level {
            parts = append(parts, topologyValue(cluster, declared))
            break
        }
        if isAncestor(declared, level) {
            parts = append(parts, topologyValue(cluster, declared))
        }
    }
    return strings.Join(parts, "/")
}
```

Selecting a non-cluster level reuses one combination routine for provider,
region, and zone:

```go
func selectGroupsAtLevel(
    groups []TopologyGroup,
    groupConstraint SpreadConstraint,
    clusterConstraint SpreadConstraint,
) ([]TopologyGroup, error) {
    if len(groups) < groupConstraint.MinGroups {
        return nil, fmt.Errorf("not enough feasible %s groups", groupConstraint.SpreadByField)
    }

    candidates := make([]GroupInfo, 0, len(groups))
    for _, group := range groups {
        candidates = append(candidates, GroupInfo{
            Name:   group.Key,
            Value:  group.UniqueClusterCount(),
            Weight: group.Score,
        })
    }

    selected := selectBestGroupCombination(
        candidates,
        groupConstraint.MinGroups,
        groupConstraint.MaxGroups,
        clusterConstraint.MinGroups,
    )
    if len(selected) == 0 {
        return nil, fmt.Errorf("not enough clusters under selected %s groups", groupConstraint.SpreadByField)
    }
    return groupsByKeys(groups, selected), nil
}
```

Final cluster selection de-duplicates multi-zone clusters before checking
`cluster.minGroups`:

```go
func finalizeClusters(
    clusters []ClusterDetailInfo,
    clusterConstraint SpreadConstraint,
    placement Placement,
) ([]ClusterDetailInfo, error) {
    uniqueClusters := deduplicateClustersByName(clusters)
    if len(uniqueClusters) < clusterConstraint.MinGroups {
        return nil, fmt.Errorf("not enough unique clusters after de-duplication")
    }

    sortClusters(uniqueClusters)
    needCount := min(len(uniqueClusters), clusterConstraint.MaxGroups)

    if requiresResourceAwareSelection(placement) {
        selected := selectClustersByAvailableResource(uniqueClusters, needCount, placement.NeedReplicas)
        if len(selected) < clusterConstraint.MinGroups {
            return nil, fmt.Errorf("not enough clusters satisfy resource requirements")
        }
        return selected, nil
    }

    return uniqueClusters[:needCount], nil
}
```

### Group Selection

Group selection should preserve the current behavior used by the existing
`region + cluster` path:

- Prefer groups with better aggregate score.
- Prefer combinations that can satisfy downstream cluster requirements.
- Respect `minGroups` and `maxGroups`.
- Keep deterministic ordering when scores and capacities are equal.

The existing group-combination search can be generalized so it receives a list
of candidate groups and a target count derived from downstream requirements.
This avoids introducing a separate hard-coded selector for each topology level.

### Cluster Finalization

Final cluster selection should reuse the existing cluster ordering principles:

- lower overflow order wins;
- higher cluster score wins;
- available replicas are considered when resource-aware selection is active;
- cluster name provides deterministic tie-breaking.

The final number of selected clusters must not exceed
`cluster.maxGroups`, and it must satisfy `cluster.minGroups`.

### Multi-Zone Cluster Handling

`ClusterSpec.Zones` supports clusters that span multiple failure zones. When a
cluster belongs to multiple zones, it may appear under multiple zone groups in
the topology tree. The final selected cluster list must still contain each
cluster at most once.

The selector should use zone membership to determine whether a zone group is
eligible, but cluster finalization must de-duplicate clusters by cluster name.
If de-duplication makes it impossible to satisfy `cluster.minGroups`, selection
must fail.

### Missing Topology Properties

The existing filter plugin rejects clusters that do not contain a required
property for a declared spread field:

- `provider` requires `cluster.spec.provider`;
- `region` requires `cluster.spec.region`;
- `zone` requires `cluster.spec.zones`.

This proposal keeps that behavior. A skipped level does not require its
property. For example, `provider + zone + cluster` does not require
`cluster.spec.region`.

### Examples

The following diagram shows three concrete scheduling examples. Blue clusters
are selected by the scheduler, gray clusters remain candidates but are not part
of the final result.

![spreadconstraints examples](spreadconstraints-examples.drawio.svg)

#### zone + cluster

In this scenario, a user has four candidate clusters distributed across two
zones. The workload should run in two zones and select two clusters.

```yaml
spreadConstraints:
- spreadByField: zone
  minGroups: 2
  maxGroups: 2
- spreadByField: cluster
  minGroups: 2
  maxGroups: 4
```

The scheduler first selects `zone-a` and `zone-b`. It then selects one cluster
from each zone, for example `member1` and `member3`, based on score, resource
availability, overflow order, and deterministic tie-breaking. Provider and
region are not part of this constraint.

#### region + zone + cluster

In this scenario, a workload should span two regions and at least three
clusters. Each selected zone is scoped by its region, so zones with the same
name in different regions are separate groups.

```yaml
spreadConstraints:
- spreadByField: region
  minGroups: 2
  maxGroups: 2
- spreadByField: zone
  minGroups: 2
  maxGroups: 4
- spreadByField: cluster
  minGroups: 3
  maxGroups: 6
```

The scheduler selects two regions, then selects zones within those selected
regions. A zone group is identified by `region/zone`, so two regions that both
contain `zone-a` do not share one global `zone-a` group. For example, the
scheduler can select `east/zone-a`, `east/zone-b`, and `west/zone-a`, then
select `member1`, `member2`, and `member3`.

#### provider + zone + cluster

In this scenario, a workload should span two providers and four clusters, but
the user does not require region spreading. This is useful when providers use
different region naming conventions or when region distribution is already
handled by affinity.

```yaml
spreadConstraints:
- spreadByField: provider
  minGroups: 2
  maxGroups: 2
- spreadByField: zone
  minGroups: 2
  maxGroups: 4
- spreadByField: cluster
  minGroups: 4
  maxGroups: 8
```

The scheduler selects two providers, then selects zones within those selected
providers. Because `region` is not declared, region is not used as a spreading
requirement. A zone group is keyed as `provider/zone`, so `provider-a/zone-a`
and `provider-b/zone-a` are different groups even though their zone names are
the same.

#### provider + region + zone + cluster

In this scenario, a high-availability workload must spread across every
topology level.

```yaml
spreadConstraints:
- spreadByField: provider
  minGroups: 2
  maxGroups: 2
- spreadByField: region
  minGroups: 2
  maxGroups: 4
- spreadByField: zone
  minGroups: 3
  maxGroups: 6
- spreadByField: cluster
  minGroups: 4
  maxGroups: 8
```

The scheduler selects providers first, then regions inside those providers,
then zones inside those provider-region paths, and finally clusters. A zone
group is keyed as `provider/region/zone`, which preserves the full hierarchy.

### Compatibility

This proposal does not change the public API. Existing policies remain valid.

The existing `cluster` behavior should remain unchanged. The existing
`region + cluster` behavior should remain compatible in normal cases because the
generic hierarchy will select regions first and then clusters from the selected
regions, matching the current model.

The main behavior change is that previously unsupported combinations will become
supported instead of failing or falling back to incomplete logic.

### Risks and Mitigations

#### Selection behavior changes for existing region constraints

Refactoring `region + cluster` into a generic selector could unintentionally
change edge-case tie-breaking.

Mitigation: preserve existing tests for `region + cluster`, add compatibility
cases, and use deterministic sorting by overflow order, score, available
replicas, and name.

#### Multi-zone clusters can make group counts misleading

A cluster that belongs to multiple zones may make several zone groups look
feasible, but final de-duplication may reduce the number of selected clusters.

Mitigation: de-duplicate at finalization and re-check `cluster.minGroups`. If
the requirement cannot be satisfied after de-duplication, fail scheduling.

#### Generic selection can be more computationally expensive

Searching combinations at multiple declared levels can be more expensive than
one specialized region selector.

Mitigation: apply the search only to declared levels, prune combinations that
cannot satisfy downstream requirements, and keep the candidate set limited to
feasible clusters after scheduler filtering.

## Test Plan

Unit tests should cover:

- constraint normalization independent of YAML order;
- topology tree construction for provider, region, zone, and cluster;
- every supported `SpreadByField` combination;
- skipped-level semantics, especially `provider + zone + cluster`;
- same zone name under different declared ancestors;
- multi-zone clusters and final cluster de-duplication;
- failures for insufficient provider, region, zone, or cluster groups;
- preservation of current `cluster` and `region + cluster` behavior;
- resource-aware final cluster selection.

Integration or end-to-end tests should cover representative user-visible
scenarios:

- spreading workloads across zones and clusters;
- spreading workloads across regions and zones;
- spreading workloads across providers, zones, and clusters without requiring
  regions;
- full provider, region, zone, and cluster spreading.

## Alternatives

### Add only the missing zone branch

This would solve `zone + cluster` and part of `region + zone + cluster`, but it
would leave provider combinations incomplete and continue the pattern of adding
special cases per combination.

### Add one selector per combination

Each combination could have its own selector, such as
`selectBestClustersByProviderAndRegion` or `selectBestClustersByRegionAndZone`.
This is straightforward for a small number of cases, but it is harder to
maintain and easier for behavior to diverge between combinations.

### Treat zone as a global group

Another option is to group zones only by zone name. Under that model, `zone-a`
in different regions or providers would be treated as the same zone. This does
not match the declared hierarchy and can produce results that unexpectedly
ignore selected ancestors. This proposal instead keys groups by declared
ancestor path.
