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

`MaxReplica` estimation will use a sum of all the resource requirement for every component's replica.
- The reason why we do not run a maxReplica estimation for each component as is - the difficulty is determining if both components can be scheduled on the same cluster. If we maintain a maxReplica estimate for each component, not only is the estimation more complex, but it is possible to run into edge cases where both components cannot fit on the same cluster even though individually they could be scheduled.
- Once the MaxReplica estimation is complete, we will return the unit to replicas by multiplying the result with `totalReplicas` field.

We will describe the maxReplica estimation in two parts:

1. The accurate estimator will create an estimate for the maxReplicas that can be scheduled. This estimate will depend on the amount of components set for the resource. If the number of components is 1, then the estimation will be done in the same way that it's done today. If the number of components is greater than 1, then we will sum up all resources required by all replicas to see how many of the total CRD can fit in the available resources.
	- `If components = 1`: maxReplicas = Math.min(Available CPU Resources / replica_cpu, Available Memory Resources / replica_memory)
	- `If components > 1`: maxReplicas = (totalReplicas) * (Math.min(Available CPU Resource / sum(replica_cpu), Available Memory Resource / sum(replica_memory))).
		- `Note`: We multiply the calculation by totalReplicas to bring the unit back to replicas. The calculation is done from the perspective of the entire CRD, but Karmada interprets replicas. That means the maxReplica calculation will always be a multiple of the number of totalReplicas.

Here is an example to illustrate the case in which components > 1. Let's assume we have a CRD with `totalReplicas` = 3, with two components = {component_1: {replicas: 1, cpu: 1, memory: 2GB}, component_2: {replicas: 2, cpu: 1, memory: 1GB}}. We are estimating maxReplica count for a target cluster with a ResourceQuota that has 6CPU available and 8GB of memory available.

During maxReplica estimation, we will take the sum of all resource requirement for the CRD.

Total_CPU = component_1.replicas * (component_1.cpu) + component_2.replicas * (component_2.cpu) = (1 * 1) + (2 * 1) = 3 CPU.
Total_Memory = component_1.replicas * (component_1.memory) + component_2.replicas * (component_2.memory) = (1 * 2GB) + (2 * 1GB) = 8GB.

Now that we have resource totals, we can calculate how many of the total CRD can fit in the available resources:

maxReplica = totalReplicas * Math.min (RQ.cpu / total_cpu, RQ.memory / total_memory) = (1 + 2) * Math.min (2, 2) = 6 replicas (or 2 total CRDs).

2. The accurate estimator will run a decision algorithm to verify that all component's replicas can fit in a combination of available nodes.
	- If the verification step returns `false`: We will return maxReplicas = 0, as the CRD cannot be fully scheduled to the cluster.
	- If the verification step returns `true`: We will return the maxReplica estimate from step 1. The estimate may not be fully accurate, but we will be certain that `at least one` full CRD can be scheduled on the target cluster.

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

#### Alternative 2: Not change current Estimator interface but extend the input and output

#### Alternative 3: Introduce a new Estimator interface dedicated for multi-components workload


### 4. Feature Gate: MultiplePodTemplatesScheduling

This feature is controlled by the feature gate `MultiplePodTemplatesScheduling`, which is disabled by default. When enabled, the scheduler and resource interpreter will use the new `GetComponentReplicas` hook to support scheduling and resource estimation for workloads with multiple pod templates or components. This allows for incremental adoption and safe experimentation with the new extensible scheduling capabilities.
