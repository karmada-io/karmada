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
- Introduce a cost-optimized scheduling strategy, where the scheduling takes cluster cost factors into consideration in this strategy.
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
### New Score Plugin `ClusterCost`
ClusterCost calls an HTTP interface to fetch the cost of the cluster and uses the **negative value of the cost** as the score. This ensures that clusters with lower costs are prioritized in descending order of scores.

Considering the cluster cost is a relatively stable figure, we can record cluster costs and their retrieval time. The HTTP interface is called only if the current time exceeds one hour from the last retrieval; otherwise, the cached data will be used.

```go
// ClusterCost is a score plugin that favors cluster that has lower cost.
type ClusterCost struct {
	clusterCosts map[string]CostInfo
}

type CostInfo struct {
	Cost        float32
	FetchedTime time.Time
}

// Score calculates the score on the candidate cluster.
// The score will be set as negative cost.
// so that in descent order, we can choose the lowest cost.
func (p *ClusterCost) Score(_ context.Context,
	_ *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {

	clusterName := cluster.Name
	currentCostInfo, exists := p.clusterCosts[clusterName]

	// Check if we need to update the cost
	if !exists || time.Since(currentCostInfo.FetchedTime) > time.Hour {
		// Fetch new cost if not exists or outdated
		cost, err := fetchCostFromAPI(cluster.Spec.APIEndpoint, clusterName)
		if err != nil {
			return 0, framework.NewResult(framework.Error, fmt.Sprintf("Error fetching cost: %v", err))
		}
		currentCostInfo = CostInfo{Cost: cost, FetchedTime: time.Now()}
		p.clusterCosts[clusterName] = currentCostInfo
	}

	var score = int64(-currentCostInfo.Cost * 100)
	return score, framework.NewResult(framework.Success)
}
```

### New Strategy
Add a new `ReplicaDivisionPreference` named `CostOptimized` and a new strategy named `CostOptimizedStrategy`.
```go
const (
	// ReplicaDivisionPreferenceAggregated divides replicas into clusters as few as possible,
	// while respecting clusters' resource availabilities during the division.
	ReplicaDivisionPreferenceAggregated ReplicaDivisionPreference = "Aggregated"
	// ReplicaDivisionPreferenceWeighted divides replicas by weight according to WeightPreference.
	ReplicaDivisionPreferenceWeighted ReplicaDivisionPreference = "Weighted"
      + // ReplicaDivisionPreferenceCostOptimized divides replicas and scheduler them to the cheapest cluster firstly.
      + ReplicaDivisionPreferenceCostOptimized ReplicaDivisionPreference = "CostOptimized"
)
```

```go
var (
	assignFuncMap = map[string]func(*assignState) ([]workv1alpha2.TargetCluster, error){
		DuplicatedStrategy:    assignByDuplicatedStrategy,
		AggregatedStrategy:    assignByDynamicStrategy,
		StaticWeightStrategy:  assignByStaticWeightStrategy,
		DynamicWeightStrategy: assignByDynamicStrategy,
              + CostOptimizedStrategy: assignByDynamicStrategy,
	}
)

const (
	// DuplicatedStrategy indicates each candidate member cluster will directly apply the original replicas.
	DuplicatedStrategy = "Duplicated"
	// AggregatedStrategy indicates dividing replicas among clusters as few as possible and
	// taking clusters' available replicas into consideration as well.
	AggregatedStrategy = "Aggregated"
	// StaticWeightStrategy indicates dividing replicas by static weight according to WeightPreference.
	StaticWeightStrategy = "StaticWeight"
	// DynamicWeightStrategy indicates dividing replicas by dynamic weight according to WeightPreference.
	DynamicWeightStrategy = "DynamicWeight"
      + // CostOptimizedStrategy indicates dividing replicas and scheduler them to the cheapest cluster firstly.
      + CostOptimizedStrategy = "CostOptimized"
)
```
When the `ReplicaSchedulingType` is `Divided` and the `ReplicaDivisionPreference` is `CostOptimized` , the strategy will be `CostOptimizedStrategy`.
```go
func newAssignState(candidates []*clusterv1alpha1.Cluster, placement *policyv1alpha1.Placement, obj *workv1alpha2.ResourceBindingSpec) *assignState {
	var strategyType string

	switch placement.ReplicaSchedulingType() {
	case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
		strategyType = DuplicatedStrategy
	case policyv1alpha1.ReplicaSchedulingTypeDivided:
		switch placement.ReplicaScheduling.ReplicaDivisionPreference {
		case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
			strategyType = AggregatedStrategy
		case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
			if placement.ReplicaScheduling.WeightPreference != nil && len(placement.ReplicaScheduling.WeightPreference.DynamicWeight) != 0 {
				strategyType = DynamicWeightStrategy
			} else {
				strategyType = StaticWeightStrategy
			}
              + case policyv1alpha1.ReplicaDivisionPreferenceCostOptimized:
              +	        strategyType = CostOptimizedStrategy
		}
	}

	return &assignState{candidates: candidates, strategy: placement.ReplicaScheduling, spec: obj, strategyType: strategyType}
}
```

As the clusters are already sorted by score in descending order, the `CostOptimizedStrategy` simply needs to fill the clusters in sequence until the required number of replicas is reached. This logic is similar to the existing `AggregatedStrategy`, so we can add `CostOptimizedStrategy` in the same switch case.
```go
func dynamicDivideReplicas(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	if state.availableReplicas < state.targetReplicas {
		return nil, &framework.UnschedulableError{Message: fmt.Sprintf("Clusters available replicas %d are not enough to schedule.", state.availableReplicas)}
	}

	switch state.strategyType {
      - case AggregatedStrategy:
      + case AggregatedStrategy, CostOptimizedStrategy:
		state.availableClusters = state.resortAvailableClusters()
		var sum int32
		for i := range state.availableClusters {
			if sum += state.availableClusters[i].Replicas; sum >= state.targetReplicas {
				state.availableClusters = state.availableClusters[:i+1]
				break
			}
		}
		fallthrough
```
### Cluster Cost Interface
We will start a cluster cost server in each cluster, and use two RESTful HTTP APIs. **_Or should we use gRPC?_**
#### 1. Get Cluster Cost
- URL: `/api/cost`
- Method: `GET`
- Description: Retrieve the cost associated with a specified cluster.
- Query Parameters
  - `cluster` (string, required): The name of the cluster to get the cost for.
- Success Response:
  - Code: 200 OK
  - Content:
  ```json
  {
    "cost": 1.11  // example value
  }
  ```
- Sample Call: `curl -X GET "http://[hostname]:[port]/api/cost?cluster=myCluster"`

#### 2. Update Cluster Cost
- URL: `/api/cost`
- Method: `PUT`
- Query Parameters
  - `cluster` (string, required): The name of the cluster to update the cost for.
  - `cost` (float, required): The new cost of the cluster, as a float value.
- Sample Call: `curl -X PUT "http://[hostname]:[port]/api/cost?cluster=myCluster&cost=2.50"`*

### Test Plan
- Unit Test covering:
    - Test `ClusterCost` score plugin.
- E2E Test covering:
    - Member cluster pricing update process.
    - Workload propagating with scheduling type `CostOptimized`.
