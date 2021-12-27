# FAQ(Frequently Asked Questions)

## What is the difference between PropagationPolicy and ClusterPropagationPolicy?

The `PropagationPolicy` is a namespace-scoped resource type which means the objects with this type must reside in a namespace.
And the `ClusterPropagationPolicy` is the cluster-scoped resource type which means the objects with this type don't have a namespace.

Both of them are used to hold the propagation declaration, but they have different capacities:
- PropagationPolicy: can only represent the propagation policy for the resources in the same namespace.
- ClusterPropagationPolicy: can represent the propagation policy for all resources including namespace-scoped and cluster-scoped resources.

## What is the difference between 'Push' and 'Pull' mode of a cluster?

Please refer to [Overview of Push and Pull](./userguide/cluster-registration.md#overview-of-cluster-mode).