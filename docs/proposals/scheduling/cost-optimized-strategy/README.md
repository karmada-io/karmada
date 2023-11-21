---
title: Cost Optimized Strategy Support
authors:
- "@realnumber666"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2023-10-26
---
## Summary

Based on the current cross-cluster scheduling, which takes into account factors like remaining resource capacity, users are increasingly concerned about costs. If workloads can be preferentially scheduled to clusters with lower prices, it can significantly reduce user expenses. However, current schedulers cannot make decisions based on the price of a cluster.

This KEP aims to add an interface to retrieve cluster prices, and during scheduling, support prioritizing workloads to be placed on clusters with lower costs.
## Motivation
At present, there's no way to optimize costs by having the Karmada cluster scheduler make decisions based on cluster prices. Therefore, we hope to enable the Karmada scheduler to be aware of cluster prices, and when users choose a cost-optimized scheduling strategy, prioritize placing workloads on the cluster with the lowest price.
### Goals
- Enable the Karmada scheduler to be aware of the cost of member clusters.
- Introduce a cost-optimized scheduling mode, where the scheduling takes cluster cost factors into consideration in this mode.
### Non-Goals
- Use public cloud pricing API to calculate cluster's price by Karmada itself.
- When the workload has already been deployed, if the cluster prices change, the scheduler will reschedule based on the new prices.

## Proposal

### User Stories (Optional)

As a cost-sensitive user, I currently have 2 clusters with different prices. I hope my workload can be scheduled in the most cost-effective way.
- Workload: requests 12 replicas, each replica requests 8 cpu.
- Cluster A: 20 nodes and each has 8 cpu remaining. The price is ¥0.5/h.
- Cluster B: 10 nodes and each has 8 cpu remaining. The price is ¥0.3/h.

In the end, 10 replicas of the workload will be created in Cluster B, and 2 replicas will be created in Cluster A. Because the price of Cluster B is lower than that of Cluster A, scheduling the workload to Cluster B as much as possible can make it more cost-effective.

## Design Details
### API Change
We will add a `price` field to the `AccurateSchedulerEstimatorServer`.
```go
type AccurateSchedulerEstimatorServer struct {  
    port            int  
    clusterName     string  
    kubeClient      kubernetes.Interface  
    restMapper      meta.RESTMapper  
    informerFactory informers.SharedInformerFactory  
    nodeLister      listv1.NodeLister  
    replicaLister   *replica.ListerWrapper  
    informerManager genericmanager.SingleClusterInformerManager  
    parallelizer    parallelize.Parallelizer  
    price           float32
  
    Cache schedcache.Cache  
}
```
### Get Member Cluster Price
We will introduce a method for the scheduler to retrieve the cluster pricing:
- The Estimator Server will pull the cluster pricing API every hour and update the `AccurateSchedulerEstimatorServer.price`.
    - Users are required to provide an HTTP interface following specific guidelines.
### New Strategy
We will introduce a new scheduling strategy named `CostOptimizedStrategy`. This strategy will have a corresponding function called `assignByCostOptimizedStrategy`, with the following steps:
- Calculate the available resource amount for each cluster.
- Retrieve the resource price for each cluster.
- Generate a struct as follows:
```go
{
	Name string `json:"name"`
	Replicas int32 `json:"replicas,omitempty"`
	Price float32 `json:"price,omitempty"`
}
```
- Sort the clusters in ascending order based on Price and fill each member cluster sequentially based on the replica demand of the workload.
- Record the results as `[]workv1alpha2.TargetCluster` and return them.

### Test Plan
- Unit Test covering:
    - Test `assignByCostOptimizedStrategy` function.
- E2E Test covering:
    - Member cluster pricing update process.
    - Workload propagating with scheduling type `CostOptimized`.
