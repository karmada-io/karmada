---
title: Cluster status-based policy orchestration solutions
authors:
- "@XiShanYongYe-Chang"
reviewers:
- "@RainbowMango"
- "@chaunceyjiang"
- TBD
approvers:
- "@RainbowMango"
- TBD

creation-date: 2024-02-19
update-date: 2024-03-04

---

# Cluster status-based policy orchestration solutions

## Summary

Currently, Karmada determines the health status of the cluster through the kube-apiserver connectivity with member clusters. This detection method has a large granularity. On the one hand, relying solely on kube-apiserver to characterize the health status of the cluster is not enough. Users must combine business needs. Sometimes it is required to know the health status of key components in the cluster, such as CoreDNS components, kube-proxy components, etc., so the abnormal situations of the cluster can be further distinguished, allowing Karmada to handle these different situations differently; on the other hand, Karmada control may occur Problems such as network jitter between the interface and the member cluster kube-apiserver cause connectivity abnormalities, but the functions within the cluster are still running normally.

Therefore, Karmada needs to provide the ability to detect component functions in member clusters without going through the kube-apiserver of the member cluster.

With the development of business, the fault descriptions of cluster functions will become more abundant, and Karmada will need to handle more behaviors. Users can customize processing policies and what actions to perform when the cluster is in which state to ensure Business availability. In order to accurately handle this type of logic, a new Remedy API is added to declare user-defined policies. Karmada system will decide whether to perform the target action based on the user-defined Remedy resources and the cluster status.

## Motivation

In order to further improve application availability, users can use Karmada to deploy applications across clusters. However, in the existing multi-cluster service access mechanism based on [single-cluster service discovery](https://github.com/karmada-io/karmada/blob/master/docs/proposals/service-discovery/README.md) and load balancing, when a key component of a member cluster fails, the entire multi-cluster service discovery capability will be unavailable.

For example, when the CoreDNS component in cluster memberA functions abnormally, even if the services and CoreDNS components in cluster memberB are healthy, access to the service backend in cluster memberB from cluster memberA will fail, and services in a certain cluster cannot be reached. Unavailable, the desired effect is to switch the business to another cluster.

Users expect that when the system detects a member cluster failure, it will automatically trigger cluster business traffic removal to ensure that business requests will not be load balanced to the failed cluster, causing service unavailability. In addition, member clusters need to upgrade the Kubernetes version. Services deployed on clusters that are in the upgrade process may be in an unavailable state. Users need to manually trigger cluster traffic removal to avoid service unavailability.

### Goals

- Provide a sample component to detect domain name functional anomalies in Kubernetes clusters. Users can use this sample to complete end-to-end capability testing.
- Provide the function to automatically trigger the removal of all MultiClusterIngress business traffic on the kubernetes cluster.
- Provide the function to manually trigger the removal of all MultiClusterIngress business traffic on the kubernetes cluster.

### Non-Goals

The detailed solution design for domain name resolution capability detection in the cluster requires in-depth analysis, and the current proposal does not involve this solution (We look forward to carrying out in-depth and detailed feature design for cluster fault detection capabilities in subsequent iterations.).

- Do not list all cluster statuses that need to be detected.
- Not all actions that the remedy policy can trigger execution are listed.

## Proposal

### User Stories (Optional)

#### Story 1

The administrator is preparing to upgrade the Kubernetes version of the member1 cluster. In order to prevent data plane failures that may be caused by cluster upgrades from affecting user services on the cluster, an immediate policy is created in the system to trigger the system to remove all business traffic on the member1 cluster to avoid End user access is affected.

#### Story 2

The administrator enables the capability by configuring remedy policies in the system: When the system detects an abnormality in the CoreNDS domain name resolution capability in the member1 cluster, the system automatically removes all MultiClusterIngress traffic on the member1 cluster to prevent end users from accessing the MultiClusterIngress backend through the LoadBalance service.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

The cluster status detection component is provided as a sample only and is not recommended for use in production environments.

When users find that the domain name resolution function in the cluster is unavailable through other tools or relying on operation and maintenance experience, they can proactively report it and synchronize the information to the condition of the Cluster object.

## Design Details

### Cluster status management

We do cluster status management by extending the ConditionType in the Cluster object. A new `ServiceDomainNameResolutionReady` ConditionType is added to represent the domain name resolution function within the cluster.

Let us show the cluster status through Yaml example.

```yaml
apiVersion: "cluster.karmada.io/v1alpha1"
kind: "Cluster"
metadata:
  name: "member1"
spec:
  apiEndpoint: "https://192.10.0.1:6443"
  syncMode: Push
status:
  conditions:
  - lastTransitionTime: "2024-01-25T04:05:57Z"
    message: "cluster is healthy and ready to accept workloads"
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-01-25T04:05:57Z"
    message: "coredns function is normal "
    status: "True"
    type: ServiceDomainNameResolutionReady
```

The above example records that the Service domain name resolution function in cluster member1 is normal.

When an exception occurs in the Service domain name resolution function in cluster member1, the cluster object is displayed as follows:

```yaml
apiVersion: "cluster.karmada.io/v1alpha1"
kind: "Cluster"
metadata:
  name: "member1"
spec:
  apiEndpoint: "https://192.10.0.1:6443"
  syncMode: Push
status:
  conditions:
  - lastTransitionTime: "2024-01-25T04:05:57Z"
    message: "cluster is healthy and ready to accept workloads"
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-01-25T04:05:57Z"
    message: "coredns function is not normal "
    status: "False"
    type: ServiceDomainNameResolutionReady
 ```

### Cluster status detection

The cluster status detection mechanism can detect active components directly without using the kube-apiserver of member clusters to avoid intermediate component failures affecting overall fault detection and response. In terms of the specific live detection effect, the active cluster group function is Availability, not just component survival.

> Note: The cluster status detection component is provided as an example only and is not recommended for use in production environments.

#### Component build and deployment

Deploy the agent component in the form of DaemonSet in the Karmada member cluster to automatically detect cluster status changes in the cluster and report the detection results to the Karmada control plane. 

When reporting detection results, it is not necessary for each instance to report, and the master selection operation is performed so that the master instance is responsible for reporting the detection results.

In order to access the Karmada control plane, the Karmada control plane access credentials parameter `--karmada-kubeconfig` needs to be added to the agent component assembly for the user to specify.

#### Domain name resolution function detection

Instances in the component will execute commands periodically (configured through the proxy component startup parameter `--detect-period`, 5s by default), and perform domain name resolution through the `nslookup kubernetes.default` command.

- If the command is executed successfully, it means that this Service domain name resolution is successful;
- If the execution of the command fails, it means that this Service domain name resolution fails.

The determination of Service's domain name resolution capability is not determined by a single command execution, the component memory will record whether the domain name resolution function is normal or not previously, assuming that the `detectionFlag` is used to indicate that the value of true means normal, and the value of false means abnormal. There are the following cases:

- If `detectionFlag` is true and this Service domain name resolution is successful, the Service domain name resolution function is determined to be normal;
- If `detectionFlag` is true and this Service domain name resolution is failed, then record the current time, execute the next command, if it lasts for a period of time (configured through the proxy component startup parameter `--detect-success-threshold`, the default is 30s) Service domain name resolution fails, then it is determined that the Service domain name resolution function is abnormal;
- If `detectionFlag` is false and this Service domain name resolution is successful, then record the current time, execute the next command, if it lasts for a period of time (configured through the proxy component startup parameter `--detect-failure-threshold`, the default is 30s) Service domain name resolution successes, then it is determined that the Service domain name resolution function is normal; 
- If `detectionFlag` is false and this Service domain name resolution is failed, the Service domain name resolution function is determined to be abnormal;

#### Reporting of detection results

For detection of Service domain name resolution, the status condition of the Cluster object in the Karmada control plane will be refreshed after each detection.

### Policy control

A new independent Remedy API is added to provide scalable capabilities, allowing users to configure matching conditions, and the system triggers action execution based on cluster status conditions.

By configuring Remedy resources, the system will automatically remove the traffic on the cluster when detecting an abnormality in the Service domain name resolution function of a cluster, that is, remove the backend service address in the faulty cluster from the LoadBalance Service to improve the global availability of the MultiClusterIngress.

In addition, administrators can manually configure immediate policies that trigger targeted actions to be executed immediately.

A Remedy consists of three elements: trigger conditions (cluster status condition), focus on subjects (cluster), and execute actions

#### API Definition

The new `Remedy` CRD is used to describe the action management strategy driven by cluster status. This resource is a Cluster-level resource. Its Group is `remedy.karmada.io` and its Version is `v1alpha1`.

> Note: Remedy CRD is pre-installed during Karmada deployment.

```go
// Remedy represents the cluster-level management strategies based on ClusterState.
type Remedy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of Remedy.
	// +required
	Spec RemedySpec `json:"spec"`
}

// RemedySpec represents the desired behavior of Remedy.
type RemedySpec struct {
	// ClusterAffinity specifies the clusters that Remedy needs to pay attention to. 
	// For clusters that meet the DecisionConditions, Actions will be preformed.    
	// If empty, all clusters will be selected.    
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`
	
	// DecisionControls indicates the decision matches of triggering 
	// the remedy system to perform the actions. 
	// As long as any one DecisionControl matches, the Actions will be preformed.
	// If empty, the Actions will be performed immediately.	
	// +optional
	DecisionMatches []DecisionMatch `json:"decisionMatches,omitempty"`
	
	// Actions specifies the actions that remedy system needs to perform. 
	// If empty, no action will be performed. 
	// +optional 
	Actions []RemedyAction `json:"actions,omitempty"`
}

// DecisionMatch represents the decision match detail of activating the remedy system.
type DecisionMatch struct {
	// ClusterConditionMatch describes the cluster condition requirement.
	// +optional
	ClusterConditionMatch *ClusterConditionRequirement `json:"clusterConditionMatch,omitempty"`
}

// ClusterConditionRequirement describes the Cluster condition requirement details.
type ClusterConditionRequirement struct {
	// ConditionType specifies the ClusterStatus condition type.
	ConditionType ConditionType `json:"conditionType"`
	// Operator represents a conditionType's relationship to a conditionStatus.
	// Valid operators are Equal, NotEqual.
	Operator ClusterConditionOperator `json:"operator"`
	// ConditionStatus specifies the ClusterStatue condition status.
	ConditionStatus string `json:"conditionStatus"`
}

// ConditionType represents the detection ClusterStatus condition type.
type ConditionType string

const (
	// ServiceDomainNameResolutionReady expresses the detection of the domain name resolution
	// function of Service in the Kubernetes cluster.
	ServiceDomainNameResolutionReady ConditionType = "ServiceDomainNameResolutionReady"
)

// ClusterConditionOperator is the set of operators that can be used in the cluster condition requirement.
type ClusterConditionOperator string

const (
	// ClusterConditionEqual means equal match.
	ClusterConditionEqual ClusterConditionOperator = "Equal"
	// ClusterConditionNotEqual means not equal match.
	ClusterConditionNotEqual ClusterConditionOperator = "NotEqual"
)

// ClusterAffinity represents the filter to select clusters.
type ClusterAffinity struct {
	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// RemedyAction represents the action type the remedy system needs to preform.
type RemedyAction string

const (
	// TrafficControl indicates that the cluster requires traffic control.
	TrafficControl RemedyAction = "TrafficControl"
)

// RemedyList contains a list of Remedy.
type RemedyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Remedy `json:"items"`
}
```

The Cluster API definition is modified as follows:

```go
// ClusterStatus contains information about the current status of a
// cluster updated periodically by cluster controller.
type ClusterStatus struct {
	...

	// RemedyActions represents the remedy actions that needs to be performed	
	// on the cluster object.	
	// +optional
	RemedyActions []string `json:"remedyActions,omitempty"`
}
```

**Q:** Do we need to record which `Remedy` a certain `RemedyAction` comes from?

**A:** I understand that adding this information will be of great help to positioning problems. How to add this information specifically? I look forward to collecting more feedback from users, and then we will discuss and analyze whether to add it and how to add it.

The MultiClusterIngress API definition is modified as follows:

```go
type MultiClusterIngress struct {
    ...
	
    // Status is the current state of the MultiClusterIngress.
    // +optional
    Status MultiClusterIngressStatus `json:"status,omitempty"`
}
// MultiClusterIngressStatus is the current state of the MultiClusterIngress.
type MultiClusterIngressStatus struct {
	networkingv1.IngressStatus `json:",inline"`

	// TrafficBlockClusters records the cluster name list that needs to perform traffic block.
	// When the cloud provider implements its multicluster-cloud-provider and refreshes
	// the service backend address to the LoadBalancer Service, it needs to filter out
	// the backend addresses in these clusters.
	// +optional
	TrafficBlockClusters []string `json:"trafficBlockClusters,omitempty"`

	// ServiceLocations records the locations of MulticlusterIngress's backend
	// Service resources. It will be set by the system controller.
	// +optional
	ServiceLocations []ServiceLocation `json:"serviceLocations,omitempty"`
}

// ServiceLocation records the locations of MulticlusterIngress's backend Service resources.
type ServiceLocation struct {
	// name is the referenced service. The service must exist in
	// the same namespace as the MultiClusterService object.
	Name string `json:"name"`

	// Clusters records the cluster list where the Service is located.
	// +optional
	Clusters []string `json:"clusters,omitempty"`
}
```

#### Yaml example display

```yaml
apiVersion: "remedy.karmada.io/v1alpha1"
kind: "Remedy"
metadata:
  name: "foo"  
spec:
  decisionMatches:
  - clusterConditionMatch:
      conditionType: "ServiceDomainNameResolutionReady"
      operator: Equal
      conditionStatus: "False"
  clusterAffinity:
    clusterNames:
      - "member1"
      - "member2"
  actions:
  - "TrafficControl"
```

The user configures a remedy resource for the `ServiceDomainNameResolutionReady` cluster conditionType, specifies the cluster to which the remedy takes effect through `clusterAffinity`, and specifies execution actions through `actions`. Based on this remedy configuration, Karmada performs corresponding actions when it recognizes the corresponding cluster status conditions. For example, when cluster member1 discovers an abnormal in `ServiceDomainNameResolutionReady`, it will execute the `TrafficControl` action according to the remedy configuration, and the entire policy processing is automated without user intervention.

Users can also specify a remedy with an empty decisionMatches to directly trigger action execution:

```yaml
apiVersion: "rededition.karmada.io/v1alpha1"
kind: "Remedy"
metadata:
  name: "foo"  
spec:
  clusterAffinity:
    clusterNames:
      - "member1"
  actions:
  - "TrafficControl"
```

The administrator performs a cluster upgrade and uses this configuration remedy to remove MultiClusterIngress traffic on the cluster to ensure overall business stability during the cluster upgrade. After the upgrade is completed, the policy is deleted and the cluster returns to normal use and can continue to accept new traffic.

#### Controller logic

Add a new remedy controller that watches the changes in the `Cluster` and `Remedy` resources, reconcile the `Cluster` object, and determine if the conditions match based on the `decisionMatches` in `Remedy` resources, set the actions to be performed into the `RemedyActions` If matched.

Q: In the remedy controller, who is the primary resource?
A: Cluster resource.
Q: What is the process for reconcile?
A: The remedy controller finds all Remedy resources related to the current Cluster object, then performs condition matching, calculates the actions that need to be performed, and then updates them to the Cluster object Status RemedyActions.
Q: When will the impact on Cluster be removed?
A: When the Remedy resource is deleted or the conditions no longer match.

When the Cluster is updated with actions, different actions will be taken care of by different processors.

For `TrafficControl` action, the multiclusteringress controller in the [multicluster-cloud-provider](https://github.com/karmada-io/multicluster-cloud-provider) repo will watch it. When the `TrafficControl` action is detected in the cluster and the service backend in the multiclusteringress object exists in the cluster, the current cluster name is added to the `trafficBlockClusters` field of the multiclusteringress object.

Then the controller calls the LoadBalance interface again to remove the backend address on the cluster that requires traffic removal from the LoadBalance, thereby achieving the purpose of business offloading.

When the cluster conditions are no longer met, the relevant values in `RemedyActions` on the Cluster and `trafficBlockClusters` on the MultiClusterIngress will be cleared, and the LoadBalance interface will be called again, and business traffic will be restored.

In addition, we also need to add a new controller in the multicluster-cloud-provider repo to maintain the cluster information of the Service backend in the multiclusteringress object. It will be used in the above calculation process.

### Test Plan

- UT cover for new add code
- E2E cover for new add case

## Alternatives

### Introduce a new API for cluster status management.

why didn't we do this?

Compared with using `Cluster` API, the newly introduced API is less convenient for users to understand and integrate with third-party systems; in addition, because it currently only detects the domain name resolution function in the cluster and there are few scene inputs, the new API designed may have insufficient scalability. For the current scenario, using Cluster objects for expansion meets the needs.
