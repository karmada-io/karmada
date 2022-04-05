# FAQ(Frequently Asked Questions)

## What is the difference between PropagationPolicy and ClusterPropagationPolicy?

The `PropagationPolicy` is a namespace-scoped resource type which means the objects with this type must reside in a namespace.
And the `ClusterPropagationPolicy` is the cluster-scoped resource type which means the objects with this type don't have a namespace.

Both of them are used to hold the propagation declaration, but they have different capacities:
- PropagationPolicy: can only represent the propagation policy for the resources in the same namespace.
- ClusterPropagationPolicy: can represent the propagation policy for all resources including namespace-scoped and cluster-scoped resources.

## What is the difference between 'Push' and 'Pull' mode of a cluster?

Please refer to [Overview of Push and Pull](./userguide/cluster-registration.md#overview-of-cluster-mode).

## Why Karmada requires `kube-controller-manager`?

`kube-controller-manager` is composed of a bunch of controllers, Karmada inherits some controllers from it 
to keep a consistent user experience and behavior. 

It's worth noting that not all controllers are needed by Karmada, for the recommended controllers please
refer to [Recommended Controllers](./userguide/configure-controllers.md#recommended-controllers).
