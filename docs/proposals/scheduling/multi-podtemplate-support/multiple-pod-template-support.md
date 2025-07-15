---
title: Multiple Pod Templates Scheduling
authors:
  - "@mszacillo"
  - "@Dyex719"
  - "@RainbowMango"
reviewers:
  - "@seanlaii"
  - "@XiShanYongYe-Chang"
  - "@zhzhuang-zju"
approvers:
  - "@kevin-wangzefeng"

creation-date: 2024-06-17
---

# Multiple Pod Templates Scheduling

## Summary

Karmada currently supports resource-aware scheduling for Custom Resources (CRDs) that have a single podTemplate, by using the `GetReplicas` interface provided by the Resource Interpreter Framework. However, many CRDs (such as FlinkDeployments) consist of multiple podTemplates or components, each with different resource requirements. The current interface cannot express these differences, leading to inaccurate scheduling estimation. This proposal aims to enhance Karmada’s scheduler and the Resource Interpreter Framework to support CRDs with multiple podTemplates/components, enabling more precise scheduling and resource estimation.

## Motivation

Karmada is used as an intelligent scheduler for complex CRDs, such as FlinkDeployments, which have multiple components (e.g., JobManager and TaskManager) with different resource requirements. The current `GetReplicas` interface in the Resource Interpreter Framework assumes all replicas are identical, which is not the case for these CRDs. This limitation prevents Karmada from accurately estimating whether a CRD can be scheduled on a target cluster, especially when using the accurate estimator with the ResourceQuota plugin.

### Goals

- Extend the Resource Interpreter Framework to support extracting resource requirements from CRDs with multiple components.
- Enable more accurate scheduling estimates for CRDs with multiple podTemplates/components.
- Provide a general solution for complex workloads such as AI, Big Data, and batch jobs that often require multiple pod templates or components.

### Non-Goals

- Scheduling components of a single CRD across multiple clusters (all components will be scheduled to the same cluster).
- Optimally packing all component replicas (Kubernetes will handle actual pod scheduling).

## Proposal

We propose to extend the scheduling logic and the Resource Interpreter Framework to support a list of `ComponentRequirements`, each specifying the name, replica count, and resource requirements for a component. The scheduler estimator will be enhanced to determine whether a target cluster can accommodate a workload with multiple pod templates (components), considering the feasibility of scheduling all components together. Additionally, the estimator will provide context and metrics that can be used for scoring clusters during scheduling, rather than simply summing resource requirements or calculating how many full CRDs fit. The estimator's logic will be improved to reflect the complexity of multi-component workloads and to support more informed scheduling decisions.

### User Stories 

#### Story 1

As a user, I want to schedule a FlinkDeployment CRD with both JobManager and TaskManager components, each with different resource requirements, and have Karmada accurately estimate if the deployment can fit on a target cluster.

![FlinkDeployment Scheduling Use Case](schedule-flink-use-case.png)

#### Story 2

As a user scheduling a FlinkDeployment, I want Karmada to provide accurate FederatedResourceQuota statistics that reflect the true resource usage of all components (such as JobManager and TaskManager). Currently, due to the lack of multiple pod-template support, resource usage is often overestimated by taking the maximum value of CPU/memory across components, rather than summing their actual requirements. Supporting multiple pod templates will enable more precise quota calculations and prevent unnecessary overestimation.

### Notes/Constraints/Caveats

- **Note:** This proposal applies to a wide range of multi-component workloads, such as [RayJob](https://github.com/ray-project/kuberay/blob/b0ddad8fa5c7c46094ee8d8003075db42bd73c44/ray-operator/apis/ray/v1alpha1/rayjob_types.go#L109), [TensorFlowJob](https://github.com/kubeflow/trainer/blob/2b39d3cbc1525dc29eb40aa58b149dbeef00aa0f/pkg/apis/kubeflow.org/v1/tensorflow_types.go#L50), and other workload types listed in [Karmada issue #5115](https://github.com/karmada-io/karmada/issues/5115), including PyTorchJob, MXJob, XGBoostJob, MPIJob, PaddleJob, SparkApplication, TrainJob, Volcano Job, and more. These workloads typically consist of multiple components, each with their own resource requirements, and benefit from fine-grained, multi-component scheduling support.

- **Constraint:** It is assumed that workloads with multiple pod templates (components) will be scheduled to only one cluster, meaning all components must be placed together in the same cluster. If there is a need to spread components across multiple clusters, the splitting of the workload should be done ahead of scheduling, potentially leveraging a future workload affinity feature (planned for future development).

### Risks and Mitigations

There are no known risks so far. This feature maintains backward compatibility and will not break previous single pod template workload scheduling or existing custom resource interpreters. In earlier Karmada versions, due to the lack of this feature, users often implemented workarounds in the GetReplicas interface of the Resource Interpreter. Migration to the new interface is recommended to take full advantage of this feature, but existing implementations will continue to function as before, albeit without the benefits of multiple pod template support.

## Design Details

### 1. Resource Interpreter Framework Extension

The Resource Interpreter Framework will be extended to allow interpreters to parse and extract multiple pod templates from arbitrary CRDs. This enables Karmada to understand the structure and requirements of complex workloads, such as those with multiple components (e.g., JobManager and TaskManager in FlinkDeployment). The framework will provide a standardized way for custom interpreters to report per-component replica counts and resource requirements, supporting more accurate scheduling and quota calculations.

```golang
// ResourceInterpreter manages both default and customized webhooks to interpret custom resource structure.
type ResourceInterpreter interface {
	// ... existing methods ...

	// GetComponentReplicas extracts the resource requirements for multiple components from the given object.
	// This interpreter hook is designed for CRDs with multiple components (e.g., FlinkDeployment), but can
	// also be used for single-component resources like Deployment.
	// If implemented, the controller will use this hook to obtain per-component replica and resource
	// requirements, and will not call GetReplicas.
	// If not implemented, the controller will fall back to GetReplicas for backward compatibility.
	// This hook will only be called when the feature gate 'MultiplePodTemplatesScheduling' is enabled.
	GetComponentReplicas(object *unstructured.Unstructured) (components []workv1alpha2.ComponentRequirements, err error)

	// ... existing methods ...
}
```

A new interpreter hook, `GetComponentReplicas`, will be introduced to the ResourceInterpreter interface. This hook is designed to extract resource requirements for multiple components from CRDs. It is primarily intended for CRDs with multiple components, but can also be used for single-component resources like Deployment. The new hook can co-exist with `GetReplicas`, and may serve as a replacement for it. The controller logic will first check if `GetComponentReplicas` is implemented; if so, it will use it to obtain per-component replica and resource requirements. If the new hook is not implemented, the controller will fall back to using `GetReplicas`. This approach ensures backward compatibility and provides a clear extension path for more complex resource types. The `ComponentRequirements` struct returned by this hook is designed to be extensible, allowing new fields to be added in the future as requirements evolve.

### 2. API Changes

This proposal introduces a new field named `Components` to the `ResourceBindingSpec` struct. The legacy fields `Replicas` and `ReplicaRequirements` are not removed or moved in this proposal; instead, they will co-exist with the new `Components` field for a period of backward compatibility.

When the feature gate `MultiplePodTemplatesScheduling` is enabled, both the legacy fields and the new `Components` field will be populated. The new `Components` field is intended to eventually replace the legacy fields, providing a more flexible and extensible way to describe multi-component workloads. During the transition period, controllers and clients should be aware that both representations may be present, and should prefer the `Components` field when the feature gate is enabled. Once the new feature is mature and widely adopted, the legacy fields may be deprecated and removed in a future version.

```go
// ResourceBindingSpec represents the expectation of ResourceBinding.
type ResourceBindingSpec struct {
    // ... existing fields ...

    // ReplicaRequirements represents the requirements required by each replica.
    // +optional
    ReplicaRequirements *ReplicaRequirements `json:"replicaRequirements,omitempty"` // (legacy field)

    // Replicas represents the replica number of the referencing resource.
    // +optional
    Replicas int32 `json:"replicas,omitempty"` // (legacy field)

    // Components defines the requirements of individual components of the resource.
    // This field is introduced to support multi-component workloads and will eventually replace the legacy fields above.
    // When the MultiplePodTemplatesScheduling feature gate is enabled, both legacy(ReplicaRequirements, Replicas) 
	// and Components will be populated for backward compatibility.
    // +optional
    Components []ComponentRequirements `json:"components,omitempty"` // new field

    // ... existing fields ...
}

// ComponentRequirements represents the requirements for a specific component in a multi-component resource.
type ComponentRequirements struct {
    // Name of this component
    Name string `json:"name,omitempty"`

    // Replicas represents the replica number of the resource's component
    // +optional
    Replicas int32 `json:"replicas,omitempty"`

    // ReplicaRequirements represents the requirements required by each replica for this component.
    // +optional
    ReplicaRequirements *ReplicaRequirements `json:"replicaRequirements,omitempty"`
    
	// Additional fields may be added in the future to support more complex requirements.
}

// ReplicaRequirements represents the requirements required by each replica.
type ReplicaRequirements struct {
	// NodeClaim represents the node claim HardNodeAffinity, NodeSelector and Tolerations required by each replica.
	// +optional
	NodeClaim *NodeClaim `json:"nodeClaim,omitempty"`

	// ResourceRequest represents the resources required by each replica.
	// +optional
	ResourceRequest corev1.ResourceList `json:"resourceRequest,omitempty"`

	// Namespace represents the resources namespaces
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// PriorityClassName represents the resources priorityClassName
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// Additional fields may be added in the future to support more complex requirements.
}
```

#### Example: ResourceBinding YAMLs and Controller/Scheduler Behavior

**When the feature gate is disabled (legacy fields only):**

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: example-legacy
spec:
  resource:
    apiVersion: apps/v1
    kind: Deployment
    namespace: default
    name: nginx
  replicas: 3
  replicaRequirements:
    resourceRequest:
      cpu: "500m"
      memory: "256Mi"
    nodeClaim:
      nodeSelector:
        disktype: ssd
  # No 'components' field present
```

*Controller/Scheduler Behavior:*
- **Controller:** Uses the legacy `GetReplicas` resource interpreter hook to obtain the desired replica count and resource requirements from the workload. It then populates the legacy `replicas` and `replicaRequirements` fields in the ResourceBinding.
- **Scheduler:** Reads the populated `replicas` and `replicaRequirements` fields to determine how many replicas to schedule and what resources each replica requires. 
- This is the current/legacy behavior and is fully backward compatible.

---

**When the feature gate is enabled (both legacy and new fields populated):**

```yaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  name: example-multicomponent
spec:
  resource:
    apiVersion: batch/v1
    kind: FlinkDeployment
    namespace: default
    name: my-flink-job
  replicas: 3 # (legacy field, still populated for compatibility)
  replicaRequirements: # (legacy field, still populated for compatibility)
    resourceRequest:
      cpu: "2"
      memory: "4Gi"
  components: # (new field, preferred when feature gate is enabled)
    - name: jobmanager
      replicas: 1
      replicaRequirements:
        resourceRequest:
          cpu: "1"
          memory: "2Gi"
    - name: taskmanager
      replicas: 2
      replicaRequirements:
        resourceRequest:
          cpu: "2"
          memory: "4Gi"
```

*Controller/Scheduler Behavior:*
- **Controller:** Populates the new `components` field anyway, but for the legacy filed depends on how many components it can get.
    - If the `GetComponentReplicas` resource interpreter hook **is implemented** for the workload type:
        - The controller uses it to obtain per-component replica counts and resource requirements.
        - If only a single component is returned, the controller also populates the legacy `replicas` and `replicaRequirements` fields for backward compatibility, treating it as a single-component workload.
        - If multiple components are returned, the returned value will be used to populate the new `components` field. The controller will then try the legacy interpreter hook `GetReplicas` to populate the legacy `replicas` and `replicaRequirements` fields as well, if possible.
    - If the `GetComponentReplicas` hook **is not implemented** for the workload type:
        - The controller falls back to the legacy `GetReplicas` hook.
        - It populates both the legacy fields (`replicas`, `replicaRequirements`) and the new `components` field (with a single component reflecting the legacy values) in the ResourceBinding for compatibility and a consistent API shape.
- **Scheduler:** First examines the `components` field to determine if the workload is single-component or multi-component. For single-component workloads, it schedules as usual using the legacy logic. For multi-component workloads, it schedules according to the new multi-component logic described in the next section.
- This enables accurate scheduling for both single and multi-component workloads while maintaining backward compatibility during the migration period.

### 3. Scheduler Changes

#### Background: Current Estimator Implementation

Besides the change to the `Resource Interpreter` and the `ResourceBinding API`, we will need to make changes to the accurate estimator's implementation.

The current [estimator interface](https://github.com/karmada-io/karmada/blob/89ddfb45877edc21335e8ab4733fcc6fb79b61d9/pkg/estimator/service/service.pb.go#L89-L93) is as follows:
```golang
// EstimatorServer is the server API for Estimator service.
type EstimatorServer interface {
	MaxAvailableReplicas(context.Context, *pb.MaxAvailableReplicasRequest) (*pb.MaxAvailableReplicasResponse, error)
	// Ignore GetUnschedulableReplicas for this proposal as it is for karmada-descheduler
	// GetUnschedulableReplicas(context.Context, *pb.UnschedulableReplicasRequest) (*pb.UnschedulableReplicasResponse, error)
}

// MaxAvailableReplicasRequest represents the request that sent by gRPC client to calculate max available replicas.
type MaxAvailableReplicasRequest struct {
	// Cluster represents the cluster name.
	// +required
	Cluster string `json:"cluster" protobuf:"bytes,1,opt,name=cluster"`
	// ReplicaRequirements represents the requirements required by each replica.
	// +required
	ReplicaRequirements ReplicaRequirements `json:"replicaRequirements" protobuf:"bytes,2,opt,name=replicaRequirements"`
}

// MaxAvailableReplicasResponse represents the response that sent by gRPC server to calculate max available replicas.
type MaxAvailableReplicasResponse struct {
	// MaxReplicas represents the max replica that the cluster can produce.
	// +required
	MaxReplicas int32 `json:"maxReplicas" protobuf:"varint,1,opt,name=maxReplicas"`
}
```

The current estimator interface is designed to handle only a single PodTemplate's resource requirements at a time. 
It accepts a `ReplicaRequirements` object representing the resource needs for one type of pod (i.e., a single component) 
and returns the maximum number of such replicas that can be accommodated in a specific cluster. 
This design does not natively support workloads composed of multiple PodTemplates (multi-component workloads), 
as it cannot simultaneously consider the combined resource requirements of multiple components when estimating capacity.

The accurate estimator in Karmada is designed to be extensible and supports running multiple plugins to calculate the maximum number of replicas that can be scheduled on a cluster. Each plugin implements a specific logic or policy for replica estimation. For example, the quota-aware plugin estimates the maximum number of replicas based on available cluster resource quotas, while other plugins may consider queue, or custom scheduling constraints in a specific member cluster. The estimator executes all enabled plugins in sequence, and the final result is determined by combining their outputs, typically by taking the minimum value among all plugin results to ensure all constraints are satisfied.

In short, the accurate estimator will calculate the maxReplica count by:
1. Running the maxReplica calculation (`maxReplicas`) for each plugin enabled by the accurate estimator.
2. The accurate estimator will then loop through all nodes and sum up the amount of replicas (`sumReplicas`) that can fit in each node. This is to account for the resource fragmentation issue.
3. The result returned will be: `Math.min(maxReplicas, sumReplicas)`.

![Accurate-Scheduler-Steps](Accurate-Scheduler-Steps.png)

#### Alternative 1: No Estimator Interface Change (Sum of all PodTemplate resource requirement when passing to estimator)

The proposed enhancement introduces a more accurate estimation method (maxReplicas) for determining how many replicas of a CRD can be scheduled on a target cluster, particularly when a CRD includes multiple components.

Currently, the estimation logic assumes each CRD has a single component. This enhancement adjusts the logic to handle multi-component CRDs more accurately by summing the resource requirements across all components.

Let:
- `AvailableCPU` = cluster’s available CPU (from ResourceQuota)
- `AvailableMemory` = cluster’s available memory (from ResourceQuota)
- `TotalReplicas` = sum of all component replicas defined in the CRD

**Case 1: Single-component CRDs (componentCount = 1)**

This follows existing behavior:

```
maxReplicas = min(AvailableCPU / replicaCPU, AvailableMemory / replicaMemory)
```

**Case 2: Multi-component CRDs (componentCount > 1)**

In this case, we compute the total resources required by the entire CRD and estimate how many full instances of the CRD can be scheduled:

```
maxCRDs = min(AvailableCPU / totalCRDCPU, AvailableMemory / totalCRDMemory)
maxReplicas = maxCRDs * TotalReplicas
```

Where:
- `totalCRDCPU` = sum of (replicas × cpu) for each component
- `totalCRDMemory` = sum of (replicas × memory) for each component

Note: We multiply by TotalReplicas to convert the number of schedulable CRDs into the equivalent replica count, since Karmada interprets results in units of replicas, not CRD instances or sets.

**Example**
Let's assume:
- TotalReplicas = 3
- Cluster ResourceQuota: 6 CPUs, 8Gi Memory
- CRD has two components:
```yaml
component_1:
  replicas: 1
  cpu: 1
  memory: 2Gi
component_2:
  replicas: 2
  cpu: 1
  memory: 1Gi
```

**Step 1: Calculate total resource requirements of one CRD**

```
totalCRDCPU = (1 * 1) + (2 * 1) = 3 CPUs
totalCRDMemory = (1 * 2Gi) + (2 * 1Gi) = 4Gi
```

**Step 2: Estimate how many full CRDs can be scheduled**

```
maxCRDs = min(6 CPUs / 3 CPUs, 8Gi / 4Gi) = min(2, 2) = 2
```

**Step 3: Convert to replica count**

```
maxReplicas = 2 CRDs * 3 replicas = 6 replicas
```

##### Bin Packing Verification Step

Optimally packing all component replicas of different resource requirement into nodes of differing sizes is an NP-hard problem. That said, we aren't interested in optimally packing (since Kubernetes will handle scheduling pods), but rather verifying that all replicas from different components of the same resource can be scheduled on the available nodes in the target cluster. This would be like the decision version of a standard bin-packing problem.

Given that we are not looking for an optimal packing, we can try an estimation for our verification. A greedy approach that has quite good performance for these types of problems is `first-fit decreasing`. We would:

1. Sort all our replicas in decreasing order (using cpu, memory, or cpu * memory)
	- We can also decide to sort nodes in decreasing order, in terms of available resources.
2. For each replica, attempt to pack it into the first node that will fit.
	- If the node can fit the replica, we continue to the next replica (and update the node's available resources to reflect the packing)
	- If the node cannot fit the replica, we move to the next available node.

If at some point, a replica cannot fit in any node, we will return false. If we reach the end of our replicas and have packed them all, we will return true without looking at other packing possibilities. The general performance would depend on how many replicas (k) we are packing and how many nodes (n) we are searching. But the average known time complexity of such an algorithm is O(k*logk).

**Summary:**  This alternative proposes enhancing the internal logic of the estimator to support multi-component workloads, without changing the existing Estimator interface. By aggregating the resource requirements of all components and verifying their fit on a cluster using a bin-packing-like algorithm, the estimator can accurately determine if the entire workload can be scheduled together. This approach maintains backward compatibility and avoids interface changes, ensuring a smooth transition for existing integrations while improving scheduling accuracy for complex resources.

#### Alternative 2: Extend Estimator Interface to Support Multiple PodTemplates

In this alternative, we propose to extend the estimator gRPC interface to natively support multi-component workloads, while maintaining backward compatibility for existing single-pod-template workloads. This is achieved by adding new fields to both the `MaxAvailableReplicasRequest` and `MaxAvailableReplicasResponse` messages, while retaining the legacy fields.

**1. Protobuf Message Changes**

```protobuf
// Backward-compatible request message
message MaxAvailableReplicasRequest {
  string cluster = 1;
  ReplicaRequirements replicaRequirements = 2; // Legacy: for single pod template workloads
  repeated ComponentReplicaRequirements components = 3; // New: for multi-component workloads
}

// Legacy: requirements for a single pod template/component
message ReplicaRequirements {
  // ... existing fields ...
}

// New: requirements for a single component in a multi-component workload
message ComponentReplicaRequirements {
  string name = 1; // Name of the component (e.g., "jobmanager", "taskmanager")
  int32 replicas = 2; // Number of replicas for this component
  ReplicaRequirements replicaRequirements = 3; // Resource requirements for each replica
}
```
The request message now supports both single and multi-component workloads. Use 'replicaRequirements' for legacy single pod template cases, and 'components' for new multi-component cases.

```protobuf
// Backward-compatible response message
message MaxAvailableReplicasResponse {
  int32 maxReplicas = 1; // Legacy: for single pod template workloads
  int32 maxSets = 2;     // New: max number of full CRD sets (all components together)
}
```
The response message provides 'maxReplicas' for legacy single pod template workloads, and 'maxSets' for the maximum number of full CRD sets (all components) that can be scheduled for multi-component workloads.

While it may seem sufficient for the scheduler to receive a simple boolean indicating whether a cluster can accommodate a multi-component CRD (i.e., "can schedule: true/false"), providing the maximum number of full CRD sets (`maxSets`) that a cluster can hold offers significant advantages:

- **Resource-Aware Scheduling Decisions:** By knowing the exact number of full CRD sets a cluster can accommodate, the scheduler can make more informed decisions. For example, it can prioritize clusters with higher available capacity, leading to better resource utilization and balancing across clusters.
- **Cluster Sorting and Ranking:** With `maxSets`, the scheduler can sort clusters by their available capacity for the workload, enabling strategies such as bin-packing, spreading, or prioritizing clusters with the most headroom.

In summary, returning the maximum number of full CRD sets (`maxSets`) provides the scheduler with richer information, enabling smarter, more efficient, and more reliable scheduling decisions than a simple boolean would allow.

**2. Go Structs (generated from protobuf):**

```golang
// Backward-compatible request struct
type MaxAvailableReplicasRequest struct {
	Cluster             string
	ReplicaRequirements *ReplicaRequirements // Legacy
	Components          []*ComponentReplicaRequirements // New
}

// Legacy: requirements for a single pod template/component
type ReplicaRequirements struct {
	// ... existing fields ...
}

// New: requirements for a single component in a multi-component workload
type ComponentReplicaRequirements struct {
	Name                string
	Replicas            int32
	ReplicaRequirements *ReplicaRequirements
}
```
The Go request struct mirrors the protobuf: use 'ReplicaRequirements' for single pod template workloads, and 'Components' for multi-component workloads.

```golang
// Backward-compatible response struct
type MaxAvailableReplicasResponse struct {
	MaxReplicas int32 // Legacy
	MaxSets     int32 // New: max number of full CRD sets (all components together)
}
```
The Go response struct provides 'MaxReplicas' for single pod template workloads and 'MaxSets' for multi-component workloads, matching the protobuf definition.

**3. How the Scheduler Invokes the Estimator**

When scheduling a workload, the scheduler needs to determine how many replicas (or sets) of the workload can be placed on each candidate cluster. To do this, it invokes the estimator service with a `MaxAvailableReplicasRequest`.

The scheduler distinguishes between single-pod-template and multi-pod-template workloads by inspecting the `ResourceBinding` object:

- For **single-pod-template workloads**, `ResourceBinding.spec.components` is either absent or has a length of 0 or 1. In this case, the scheduler populates the legacy `ReplicaRequirements` field in the estimator request.
- For **multi-pod-template workloads**, `ResourceBinding.spec.components` has a length greater than 1. The scheduler constructs the `Components` field in the estimator request, filling in each `ComponentReplicaRequirements` with the requirements for each pod template/component.

Additionally, by examining the `SpreadConstraints` in ResourceBinding, the scheduler can confirm that the multi-pod-template workload is intended to be scheduled onto a single cluster. With this confirmation, the scheduler can safely invoke the new estimator interface designed for multi-component workloads.

**Example flow:**
1. The scheduler examines the `ResourceBinding` to determine if the workload is single or multi-pod-template.
2. It builds the appropriate estimator request (`ReplicaRequirements` for single, `Components` for multi).
3. The scheduler sends the request to the estimator service for each candidate cluster.
4. The estimator responds with either `maxReplicas` (single) or `maxSets` (multi), which the scheduler uses to make placement decisions.

This approach ensures that the scheduler can seamlessly support both legacy and advanced multi-component workloads, leveraging the information available in `ResourceBinding` and the referenced workload spec to drive accurate and efficient scheduling decisions.

- For single-pod-template workloads, only the legacy fields (`ReplicaRequirements` in the request, `MaxReplicas` in the response) are used.
- For multi-component workloads, the new fields (`components` in the request, `MaxSets` in the response) are used.
- The estimator implementation should detect which fields are populated and process accordingly, ensuring backward compatibility.


**4. EstimateReplicasPlugin Interface Changes**

The current [EstimateReplicasPlugin interface](https://github.com/karmada-io/karmada/blob/51aec83d125afcb96e90a31c6a381b1bc81646c1/pkg/estimator/server/framework/interface.go#L49-L59) is defined as follows:
```golang
// EstimateReplicasPlugin is an interface for replica estimation plugins.
// These estimators are used to estimate the replicas for a given pb.ReplicaRequirements
type EstimateReplicasPlugin interface {
	Plugin
	// Estimate is called for each MaxAvailableReplicas request.
	// It returns an integer and an error
	// The integer representing the number of calculated replica for the given replicaRequirements
	// The Result contains code, reasons and error
	// it is merged from all plugins returned result codes
	Estimate(ctx context.Context, snapshot *schedcache.Snapshot, replicaRequirements *pb.ReplicaRequirements) (int32, *Result)
}
```
To support multi-component (multi-pod-template) workloads, we add a new method to the EstimateReplicasPlugin interface.
This method, EstimateComponents, allows the estimator to receive a slice of ComponentReplicaRequirements and return
the maximum number of full sets (i.e., all components together) that can be scheduled on the cluster.

type EstimateReplicasPlugin interface {
	Plugin
	// Estimate is called for each MaxAvailableReplicas request for single-component workloads.
	// It returns the number of replicas that can be scheduled for the given replicaRequirements.
	Estimate(ctx context.Context, snapshot *schedcache.Snapshot, replicaRequirements *pb.ReplicaRequirements) (int32, *Result)

	// EstimateComponents is called for multi-component workloads.
	// It receives a slice of ComponentReplicaRequirements, each describing the requirements for a component (pod template).
	// It returns the maximum number of full sets (i.e., all components together) that can be scheduled on the cluster,
	// and a Result object with details.
	EstimateComponents(ctx context.Context, snapshot *schedcache.Snapshot, components []*pb.ComponentReplicaRequirements) (int32, *Result)
}

Explanation:
- The new EstimateComponents method enables the estimator to handle workloads with multiple pod templates/components.
- It takes a slice of ComponentReplicaRequirements, one for each component, and returns the maximum number of sets
  that can be scheduled given the cluster's resources and constraints.
- This preserves backward compatibility: single-component workloads use the existing Estimate method, while multi-component
  workloads use the new EstimateComponents method.

**5. How the estimator works**

The estimator's core responsibility is to determine how many replicas (for single-component workloads) or how many full sets (for multi-component workloads) can be scheduled on a given cluster, based on the available resources and the requirements of each component.

For workloads with a single pod template (i.e., single-component workloads), the estimator's logic remains unchanged:

- The estimator receives a request containing the `ReplicaRequirements` for the workload.
- It calculates the maximum number of replicas that can be scheduled on the cluster, considering resource requests, node selectors, affinity, taints, and other constraints.
- The result is returned as a single integer value (`maxReplicas`).

For workloads with multiple pod templates (i.e., multi-component workloads), the estimator follows a set-based approach:

   - The estimator receives a list of `ComponentReplicaRequirements`, each describing the resource and scheduling requirements for a component (pod template) and the number of replicas required for that component in a single set.
   - The estimator attempts to place one full set of components at a time.
   - For each set:
     - It iterates through each component in the set.
     - For each component, it tries to schedule the required number of replicas (as specified in the component's requirements) onto the cluster, considering all constraints.
     - If all components in the set can be scheduled (i.e., the cluster has enough resources and satisfies all constraints for every component in the set), the estimator "reserves" those resources and proceeds to the next set.
     - If any component in the set cannot be scheduled (i.e., insufficient resources or unsatisfiable constraints), the iteration stops.
   - The total number of full sets that could be scheduled before running out of resources or hitting constraints is returned as the result (`maxSets`).

This approach ensures that the estimator only counts complete sets (i.e., all components together) that can be scheduled, rather than overestimating by considering components independently. It accurately reflects the real-world requirement that a multi-component workload can only be deployed if all its components can be scheduled together.

**Summary Table:**

| Workload Type         | Estimator Input                | Estimator Output         | Scheduling Logic                         |
|---------------------- |-------------------------------|--------------------------|------------------------------------------|
| Single pod template   | ReplicaRequirements           | maxReplicas (int)        | As before: max number of replicas        |
| Multiple pod templates| [ComponentReplicaRequirements]| maxSets (int)            | Max number of full sets (all components) |

This set-based estimation logic provides accurate, fair, and predictable scheduling for complex workloads with multiple pod templates, while maintaining full backward compatibility for legacy single-pod-template workloads.

---

**Benefits:**
- Native support for multi-component workloads.
- Full backward compatibility for existing single-pod-template workloads.
- No need to flatten or sum resource requirements before calling the estimator.
- More accurate and flexible scheduling estimation for complex CRDs.

**Drawbacks:**
- Requires changes to the estimator interface and all implementations.
- Clients and servers must be updated to use the new fields for multi-component workloads.

---

**Summary:**  
This approach provides a clean, extensible, and backward-compatible way to support advanced scheduling scenarios involving multiple pod templates or components. By introducing new fields for multi-component support and retaining legacy fields, the API remains clear and robust for both current and future use cases.

#### Alternative 3: Introduce a new Estimator interface dedicated for multi-component workloads

In this alternative, instead of extending the existing `EstimatorServer` interface method signatures by enhancing the request and response messages, we introduce a new, dedicated gRPC service interface specifically for multi-component (multi-pod-template) workloads. This approach provides a clear separation between the legacy single-component estimation logic and the new multi-component estimation logic, making the API surface more explicit and easier to evolve independently.

**1. Proposed gRPC Interface:**
```protobuf
// MultiComponentEstimatorServer is a new gRPC service dedicated to estimating the maximum number of full sets
// of multi-component (multi-pod-template) workloads that can be scheduled on a cluster.
service MultiComponentEstimator {
  // MaxAvailableComponentSets calculates the maximum number of full sets (i.e., all components together)
  // that can be scheduled on the given cluster, based on the requirements of each component.
  rpc MaxAvailableComponentSets(MaxAvailableComponentSetsRequest) returns (MaxAvailableComponentSetsResponse);
}

// MaxAvailableComponentSetsRequest represents the request sent by the client to estimate the maximum number
// of full sets of components that can be scheduled.
message MaxAvailableComponentSetsRequest {
  // Cluster represents the cluster name.
  string cluster = 1;
  // Components contains the requirements for each component (pod template) in the workload.
  repeated ComponentReplicaRequirements components = 2;
}

// ComponentReplicaRequirements describes the resource and scheduling requirements for a single component.
message ComponentReplicaRequirements {
  // Name of the component (e.g., "jobmanager", "taskmanager").
  string name = 1;
  // Resource requirements for each replica of this component.
  ReplicaRequirements replicaRequirements = 2;
  // Number of replicas required for this component in a single set.
  int32 replicas = 3;
}

// Legacy: requirements for a single pod template/component
message ReplicaRequirements {
  // ... existing fields ...
}


// MaxAvailableComponentSetsResponse represents the response from the estimator.
message MaxAvailableComponentSetsResponse {
  // The maximum number of full sets of all components that can be scheduled on the cluster.
  int32 maxSets = 1;
}
```
This interface defines a new gRPC service, `MultiComponentEstimator`, designed specifically to estimate the scheduling capacity for workloads composed of multiple components (such as those with multiple pod templates). Unlike the legacy estimator, which operates on single-component workloads, this interface enables the scheduler to determine how many complete sets of all required components can be scheduled together on a cluster, taking into account the resource and replica requirements of each component. This separation ensures clear, maintainable, and extensible support for advanced multi-component scheduling scenarios.


**2. Go struct representations of the above protobuf interfaces**
```golang
// MaxAvailableComponentSetsRequest represents the request sent by the client to estimate the maximum number
// of full sets of components that can be scheduled.
type MaxAvailableComponentSetsRequest struct {
	// Cluster represents the cluster name.
	Cluster string `json:"cluster" protobuf:"bytes,1,opt,name=cluster"`
	// Components contains the requirements for each component (pod template) in the workload.
	Components []ComponentReplicaRequirements `json:"components" protobuf:"bytes,2,rep,name=components"`
}

// ComponentReplicaRequirements describes the resource and scheduling requirements for a single component.
type ComponentReplicaRequirements struct {
	// Name of the component (e.g., "jobmanager", "taskmanager").
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Resource requirements for each replica of this component.
	ReplicaRequirements ReplicaRequirements `json:"replicaRequirements" protobuf:"bytes,2,opt,name=replicaRequirements"`
	// Number of replicas required for this component in a single set.
	Replicas int32 `json:"replicas" protobuf:"varint,3,opt,name=replicas"`
}

// ReplicaRequirements represents the requirements required by each replica.
// (Assume this struct is already defined elsewhere, as in the legacy estimator interface.)
// type ReplicaRequirements struct {
//     // ... existing fields ...
// }

// MaxAvailableComponentSetsResponse represents the response from the estimator.
type MaxAvailableComponentSetsResponse struct {
	// The maximum number of full sets of all components that can be scheduled on the cluster.
	MaxSets int32 `json:"maxSets" protobuf:"varint,1,opt,name=maxSets"`
}
```

The above Go structures define the request and response types for the multi-component estimator service. 
- `MaxAvailableComponentSetsRequest` allows the client to specify a cluster and a list of component requirements, 
  where each component is described by its name, per-replica resource requirements, and the number of replicas per set.
- `ComponentReplicaRequirements` encapsulates the requirements for a single component within the workload.
- `MaxAvailableComponentSetsResponse` returns the maximum number of complete sets of all specified components 
  that can be scheduled on the target cluster, enabling accurate scheduling for complex, multi-component workloads.

**3. How the Scheduler Invokes the Estimator**

The scheduler's invocation of the estimator interface for multi-component workloads closely mirrors the approach described in **Alternative 2** above, with a focus on backward compatibility and clear separation between single and multi-component logic.

1. **Workload Inspection:**  
   When scheduling a workload, the scheduler first inspects the `ResourceBinding` object to determine whether the workload is single-component (legacy) or multi-component (multiple pod templates). This is typically done by checking the presence and length of the `components` field in `ResourceBinding.spec`.

2. **Request Construction:**  
   - **Single-Component Workloads:**  
     If the workload is single-component (i.e., `components` is absent or has length 0 or 1), the scheduler constructs a legacy estimator request using the `ReplicaRequirements` field and invokes the legacy estimator interface.
   - **Multi-Component Workloads:**  
     If the workload is multi-component (i.e., `components` has length > 1), the scheduler constructs a `MaxAvailableComponentSetsRequest`, populating the `components` field with a list of `ComponentReplicaRequirements` (one for each pod template/component in the workload).

3. **Estimator Invocation:**  
   - For single-component workloads, the scheduler calls the legacy estimator service (e.g., `MaxAvailableReplicas`) and uses the returned `maxReplicas` value.
   - For multi-component workloads, the scheduler calls the new multi-component estimator service (e.g., `MaxAvailableComponentSets`) and uses the returned `maxSets` value, which indicates the maximum number of *full sets* (i.e., all components together) that can be scheduled on the cluster.

4. **Scheduling Decision:**  
   The scheduler uses the estimator's response to inform placement decisions:
   - For single-component workloads, it schedules up to `maxReplicas` on the cluster.
   - For multi-component workloads, it schedules up to `maxSets` *sets* of the workload, ensuring that all required components are placed together as a unit.

**Example Flow:**
- The scheduler receives a `ResourceBinding` for a multi-component workload (e.g., a FlinkDeployment with jobmanager and taskmanager).
- It builds a `MaxAvailableComponentSetsRequest` with the per-component requirements.
- It sends this request to the estimator service for each candidate cluster.
- The estimator responds with the maximum number of full sets (`maxSets`) that can be scheduled.
- The scheduler uses this information to rank clusters, select placement, and assign the appropriate number of sets to each cluster.

This approach ensures:
- **Backward compatibility:** Single-component workloads continue to use the legacy estimator interface.
- **Extensibility:** Multi-component workloads benefit from the new estimator interface, enabling accurate, resource-aware scheduling for complex scenarios.
- **Unified scheduling logic:** The scheduler's code path is cleanly separated based on the structure of the workload, making it easy to maintain and extend.

In summary, the scheduler dynamically selects the appropriate estimator interface and request format based on the workload's structure, enabling seamless support for both legacy and advanced multi-component scheduling scenarios.

**4. EstimateReplicasPlugin Interface Changes**

The changes to the `EstimateReplicasPlugin` interface are similar to **Alternative 2** in this proposal: rather than replacing the existing `Estimate` method, we extend the interface by adding a new method, `EstimateComponents`, specifically for multi-component (multi-pod-template) workloads. This approach preserves backward compatibility for single-component workloads while enabling native support for advanced multi-component scheduling scenarios. Implementations of the interface can choose to support either or both methods, and the scheduler will invoke the appropriate method based on the structure of the workload (single or multiple components).


### 4. Feature Gate: MultiplePodTemplatesScheduling

This feature is controlled by the feature gate `MultiplePodTemplatesScheduling`, which is disabled by default. When enabled, the scheduler and resource interpreter will use the new `GetComponentReplicas` hook to support scheduling and resource estimation for workloads with multiple pod templates or components. This allows for incremental adoption and safe experimentation with the new extensible scheduling capabilities.
