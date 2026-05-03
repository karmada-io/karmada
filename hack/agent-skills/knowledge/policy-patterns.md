# Karmada Policy Patterns

## Pattern 1: Basic Propagation

Propagate a Deployment to specific clusters by name.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: basic-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-app
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

When to use: Simple static placement to known cluster names.

## Pattern 2: Label-Based Propagation

Select target clusters by label instead of name.

```yaml
placement:
  clusterAffinity:
    labelSelector:
      matchLabels:
        region: us-west
    fieldSelector:
      matchExpressions:
        - key: provider
          operator: In
          values:
            - aws
```

When to use: Dynamic cluster selection based on cluster metadata.

## Pattern 3: Spread Across Regions/Zones

Spread replicas across availability domains.

```yaml
placement:
  spreadConstraints:
    - spreadByField: region
      maxGroups: 2
      minGroups: 2
  replicaScheduling:
    replicaSchedulingType: Divided
    replicaDivisionPreference: Weighted
```

When to use: High-availability deployments across failure domains.

## Pattern 4: Weighted Replica Distribution

Assign more replicas to larger clusters.

```yaml
placement:
  clusterAffinity:
    clusterNames:
      - member1
      - member2
  replicaScheduling:
    replicaSchedulingType: Divided
    replicaDivisionPreference: Weighted
    weightPreference:
      staticWeightList:
        - targetCluster:
            clusterNames:
              - member1
          weight: 2
        - targetCluster:
            clusterNames:
              - member2
          weight: 1
```

When to use: Uneven cluster capacities, canary traffic splitting.

## Pattern 5: Image Override

Use different container images per cluster.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
  name: image-override
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-app
  overrideRules:
    - targetCluster:
        clusterNames:
          - member2
      overriders:
        imageOverrider:
          - component: Registry
            operator: replace
            value: registry.member2.example.com
```

When to use: Different image registries per region/cluster.

## Pattern 6: Image Override by Predicate

Override only specific container images within a multi-container Pod.

```yaml
overriders:
  imageOverrider:
    - predicate:
        path: /spec/template/spec/containers/0/image
      component: Registry
      operator: replace
      value: private-registry.example.com
```

When to use: Multi-container Pods where only some images need overriding.

## Pattern 7: Labels and Annotations Override

Add environment-specific labels per cluster.

```yaml
overrideRules:
  - targetCluster:
      clusterNames:
        - member1
    overriders:
      labelsOverrider:
        - operator: add
          value:
            env: production
      annotationsOverrider:
        - operator: add
          value:
            team: platform
```

When to use: Environment-specific metadata without changing the workload itself.

## Pattern 8: Command/Args Override

Change container entry point per cluster.

```yaml
overrideRules:
  - targetCluster:
      clusterNames:
        - member2
    overriders:
      commandOverrider:
        - containerName: main
          operator: add
          value:
            - "--debug"
            - "--verbose"
```

When to use: Debug clusters, different configuration flags per environment.

## Pattern 9: Plaintext Field Override

Override arbitrary fields in any resource.

```yaml
overrideRules:
  - targetCluster:
      clusterNames:
        - member1
    overriders:
      plaintext:
        - path: /spec/replicas
          operator: replace
          value: 5
```

When to use: Replica tuning per cluster, ConfigMap values, any JSONPath field.

## Pattern 10: Failover with Graceful Eviction

Configure application and cluster failover.

```yaml
spec:
  failover:
    application:
      decisionConditions:
        tolerationSeconds: 300
      purgeMode: Gracefully
      gracePeriodSeconds: 600
    cluster:
      purgeMode: Gracefully
```

When to use: Disaster recovery scenarios requiring automatic migration.

## Pattern 11: Dependency Propagation

Auto-propagate referenced resources (ConfigMaps, Secrets).

```yaml
spec:
  propagateDeps: true
```

When to use: Reduce manual config by propagating dependencies alongside the main workload.

## Pattern 12: Multi-Group Affinity with Overflow

Define primary and backup cluster groups.

```yaml
placement:
  clusterAffinities:
    - affinityName: primary
      clusterNames:
        - member1
        - member2
      overflowAffinities:
        - affinityName: overflow-1
          clusterNames:
            - member3
```

When to use: Private datacenter primary + cloud burst overflow.

## Pattern 13: Suspension

Temporarily stop or selectively suspend dispatching.

```yaml
spec:
  suspension:
    dispatching: true  # stop all

# OR per-cluster:
spec:
  suspension:
    dispatchingOnClusters:
      clusterNames:
        - member1
```

When to use: Maintenance windows, controlled rollouts.

## Pattern 14: Priority and Preemption

Define policy priority and preemption behavior.

```yaml
spec:
  priority: 100
  preemption: Always
```

When to use: Ensure critical workloads claim resources over lower-priority ones.

## Pattern 15: Conflict Resolution

Overwrite existing resources on target clusters.

```yaml
spec:
  conflictResolution: Overwrite
```

When to use: Migrating legacy cluster resources to Karmada management.

## Pattern 16: Preserve Resources on Deletion

Keep resources on member clusters when the Karmada resource template is deleted.

```yaml
spec:
  preserveResourcesOnDeletion: true
```

When to use: Safe rollback during migration where member workloads should not be deleted.

## Pattern 17: Workload Affinity/Anti-Affinity

Co-locate or separate workloads across clusters.

```yaml
placement:
  workloadAffinity:
    affinity:
      groupByLabelKey: app.group
    antiAffinity:
      groupByLabelKey: app.group
```

When to use: Microservice co-location for latency, or separation for fault isolation.
