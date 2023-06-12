---
title: CronFederatedHPA
authors:
- "@jwcesign"
reviewers:
- "@GitHubxsy"
- "@RainbowMango"
- "@chaunceyjiang"
- "@sbdtu5498"
- "@Poor12"
approvers:
- "@RainbowMango"

creation-date: 2023-06-06

---

# CronFederatedHPA
## Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap. 

A good summary is probably at least a paragraph in length.
-->

For some certain predictable load change scenarios, for example Black Friday sale and Singles' Day sale, there will be sharp load peaks. With standard K8s HPA, it needs time to scale up the replicas, which may not be fast enough to handle sharp load peaks, resulting in service unavailability.

To solve this problem, there are already some approaches in single cluster, called `CronHPA`, which can scale up/down the workloads at specific time. Cluster administrators can scale up the workloads in advance with `CronHPA`, to meet the requirements of the sharp load peaks. So this proposal proposes `CronFederatedHPA` to meet the the same requirements in multi-cluster scenario.

## Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->

The scaling speed is not fast enough to handle sharp load peaks, causing service unavailability, which is unacceptable. So the cluster administrators want to scale up/down the workloads in advance to meet the sharp load peaks in multi-cluster scenario, ensure the service is always available. This is critical in some special days, such as Black Friday sale and Singles' Day sale.

### Goals

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

1. Define the `CronFederatedHPA` API, to scale the workloads in specific time. 
1. Support both `FederatedHPA` and workloads(`Deployment`, `StatefulSet` or other resources that have a subresource named `scale`).
1. Propose the implementation ideas for involved components, including `karmada-controller-manager`.

### Non-Goals

<!--
What is out of scope for this KEP? Listing non-goals helps to focus discussion
and make progress.
-->

## Proposal

<!--
This is where we get down to the specifics of what the proposal is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?
The "Design Details" section below is for the real
nitty-gritty.
-->

### User Stories(Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story 1
As a cluster administrator, I know that there will be a sudden peak traffic every days at 9 o'clock AM. So I want to scale up the related service in advance(like 30 minutes) to meet the sharp load peaks, ensure the service is available when the time comes. Also, after the sharp load peaks have gone(like 2 hours latter), I want to scale down the related service to save cloud costs.


### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go into as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate them? 

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

`CronFederatedHPA` could scale FederatedHPA, but also scale the workloads directly.
If the workloads are scaled by `FederatedHPA`, but you configure `CronFederatedHPA` to scale the workloads at the same time, it will have conflicts as follows:  
<image src="./statics/cronfederatedhpa-constraints.png" width="300px">  

So, please ensure that the workloads will not be scale by `FederatedHPA` and `CronFederatedHPA` at the same time.

## Design Details
### Architecture
<image src="./statics/cronfederatedhpa-architecture.png" width="300px">

From the above architecture diagram, CronFederatedHPA should do following things:
* `CronFederatedHPA` allow users to scale the workloads at specific time.
* `CronFederatedHPA` allow users to set the `FederatedHPA` at specific time.

### API Definition

In order to satisfy the above requirements, we defined the API as following:
```go
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cronfhpa,categories={karmada-io}
// +kubebuilder:printcolumn:JSONPath=`.spec.scaleTargetRef.kind`,name=`REFERENCE-KIND`,type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.scaleTargetRef.name`,name=`REFERENCE-NAME`,type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name=`AGE`,type=date

// CronFederatedHPA represents a collection of repeating schedule to scale
// replica number of a specific workload. It can scale any resource implementing
// the scale subresource as well as FederatedHPA.
type CronFederatedHPA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the CronFederatedHPA.
	// +required
	Spec CronFederatedHPASpec `json:"spec"`

	// Status is the current status of the CronFederatedHPA.
	// +optional
	Status CronFederatedHPAStatus `json:"status"`
}

// CronFederatedHPASpec is the specification of the CronFederatedHPA.
type CronFederatedHPASpec struct {
	// ScaleTargetRef points to the target resource to scale.
	// Target resource could be any resource that implementing the scale
	// subresource like Deployment, or FederatedHPA.
	// +required
	ScaleTargetRef autoscalingv2.CrossVersionObjectReference `json:"scaleTargetRef"`

	// Rules contains a collection of schedules that declares when and how
	// the referencing target resource should be scaled.
	// +required
	Rules []CronFederatedHPARule `json:"rules"`
}

// CronFederatedHPARule declares a schedule as well as scale actions.
type CronFederatedHPARule struct {
	// Name of the rule.
	// Each rule in a CronFederatedHPA must have a unique name.
	//
	// Note: the name will be used as an identifier to record its execution
	// history. Changing the name will be considered as deleting the old rule
	// and adding a new rule, that means the original execution history will be
	// discarded.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	// +required
	Name string `json:"name"`

	// Schedule is the cron expression that represents a periodical time.
	// The syntax follows https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#schedule-syntax.
	// +required
	Schedule string `json:"schedule"`

	// TargetReplicas is the target replicas to be scaled for resources
	// referencing by ScaleTargetRef of this CronFederatedHPA.
	// Only needed when referencing resource is not FederatedHPA.
	// +optional
	TargetReplicas *int32 `json:"targetReplicas,omitempty"`

	// TargetMinReplicas is the target MinReplicas to be set for FederatedHPA.
	// Only needed when referencing resource is FederatedHPA.
	// TargetMinReplicas and TargetMaxReplicas can be specified together or
	// either one can be specified alone.
	// nil means the MinReplicas(.spec.minReplicas) of the referencing FederatedHPA
	// will not be updated.
	// +optional
	TargetMinReplicas *int32 `json:"targetMinReplicas,omitempty"`

	// TargetMaxReplicas is the target MaxReplicas to be set for FederatedHPA.
	// Only needed when referencing resource is FederatedHPA.
	// TargetMinReplicas and TargetMaxReplicas can be specified together or
	// either one can be specified alone.
	// nil means the MaxReplicas(.spec.maxReplicas) of the referencing FederatedHPA
	// will not be updated.
	// +optional
	TargetMaxReplicas *int32 `json:"targetMaxReplicas,omitempty"`

	// Suspend tells the controller to suspend subsequent executions.
	// Defaults to false.
	//
	// +kubebuilder:default=false
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// TimeZone for the giving schedule.
	// If not specified, this will default to the time zone of the
	// karmada-controller-manager process.
	// Invalid TimeZone will be rejected when applying by karmada-webhook.
	// see https://en.wikipedia.org/wiki/List_of_tz_database_time_zones for the
	// all timezones.
	// +optional
	TimeZone *string `json:"timeZone,omitempty"`

	// SuccessfulHistoryLimit represents the count of successful execution items
	// for each rule.
	// The value must be a positive integer. It defaults to 3.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	// +optional
	SuccessfulHistoryLimit *int32 `json:"successfulHistoryLimit,omitempty"`

	// FailedHistoryLimit represents the count of failed execution items for
	// each rule.
	// The value must be a positive integer. It defaults to 3.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	FailedHistoryLimit *int32 `json:"failedHistoryLimit,omitempty"`
}

// CronFederatedHPAStatus represents the current status of a CronFederatedHPA.
type CronFederatedHPAStatus struct {
	// ExecutionHistories record the execution histories of CronFederatedHPARule.
	// +optional
	ExecutionHistories []ExecutionHistory `json:"executionHistories,omitempty"`
}

// ExecutionHistory records the execution history of specific CronFederatedHPARule.
type ExecutionHistory struct {
	// RuleName is the name of the CronFederatedHPARule.
	// +required
	RuleName string `json:"ruleName"`

	// NextExecutionTime is the next time to execute.
	// Nil means the rule has been suspended.
	// +optional
	NextExecutionTime *metav1.Time `json:"nextExecutionTime,omitempty"`

	// SuccessfulExecutions records successful executions.
	// +optional
	SuccessfulExecutions []SuccessfulExecution `json:"successfulExecutions,omitempty"`

	// FailedExecutions records failed executions.
	// +optional
	FailedExecutions []FailedExecution `json:"failedExecutions,omitempty"`
}

// SuccessfulExecution records a successful execution.
type SuccessfulExecution struct {
	// ScheduleTime is the expected execution time declared in CronFederatedHPARule.
	// +required
	ScheduleTime *metav1.Time `json:"scheduleTime"`

	// ExecutionTime is the actual execution time of CronFederatedHPARule.
	// Tasks may not always be executed at ScheduleTime. ExecutionTime is used
	// to evaluate the efficiency of the controller's execution.
	// +required
	ExecutionTime *metav1.Time `json:"executionTime"`

	// AppliedReplicas is the replicas have been applied.
	// It is required if .spec.rules[*].targetReplicas is not empty.
	// +optional
	AppliedReplicas *int32 `json:"appliedReplicas,omitempty"`

	// AppliedMaxReplicas is the MaxReplicas have been applied.
	// It is required if .spec.rules[*].targetMaxReplicas is not empty.
	// +optional
	AppliedMaxReplicas *int32 `json:"appliedMaxReplicas,omitempty"`

	// AppliedMinReplicas is the MinReplicas have been applied.
	// It is required if .spec.rules[*].targetMinReplicas is not empty.
	// +optional
	AppliedMinReplicas *int32 `json:"appliedMinReplicas,omitempty"`
}

// FailedExecution records a failed execution.
type FailedExecution struct {
	// ScheduleTime is the expected execution time declared in CronFederatedHPARule.
	// +required
	ScheduleTime *metav1.Time `json:"scheduleTime"`

	// ExecutionTime is the actual execution time of CronFederatedHPARule.
	// Tasks may not always be executed at ScheduleTime. ExecutionTime is used
	// to evaluate the efficiency of the controller's execution.
	// +required
	ExecutionTime *metav1.Time `json:"executionTime"`

	// Message is the human-readable message indicating details about the failure.
	// +required
	Message string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CronFederatedHPAList contains a list of CronFederatedHPA.
type CronFederatedHPAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronFederatedHPA `json:"items"`
}
```

To better understand what is `status.executionHistories[*].NextExecutionTime`, here are some examples:
* The schedule time is `3 * * * *`, I apply this rule when 9:04 AM, so `status.executionHistories[*].NextExecutionTime` is: 10:03 AM, after executing this rule, the next time will be updated to 11:03 AM.
* The schedule time is `3 * * * *`, I apply this rule when 9:01 AM, so `status.executionHistories[*].NextExecutionTime` is: 09:03 AM, after executing this rule, the next time will be updated to 10:03 AM.

### Components change

#### karmada-controller-manager  
In order to make `CronFederatedHPA` work, CronFederatedHPA controller should be implemented in `karmada-controller-manager`, it will check the time and apply the rules of `CronFederatedHPA` at the configured time. The timestamp is configured via `spec.rules[*].schedule`, which should restrict the following format:
```sh
# ┌───────────── minute (0 - 59)
# │ ┌───────────── hour (0 - 23)
# │ │ ┌───────────── day of the month (1 - 31)
# │ │ │ ┌───────────── month (1 - 12)
# │ │ │ │ ┌───────────── day of the week (0 - 6) (Sunday to Saturday;
# │ │ │ │ │                                   7 is also Sunday on some systems)
# │ │ │ │ │                                   OR sun, mon, tue, wed, thu, fri, sat
# │ │ │ │ │
# * * * * *
```
For example:
| Description																									| Equivalent to |
| ------------- 																							|-------------  |
| Run once a year at midnight of 1 January										| 0 0 1 1 * 		|
| Run once a month at midnight of the first day of the month	| 0 0 1 * * 		|
| Run once a week at midnight on Sunday morning								| 0 0 * * 0 		|
| Run once a day at midnight																	| 0 0 * * * 		|
| Run once an hour at the beginning of the hour								| 0 * * * * 		|

#### karmada-webhook
In order to make sure the applied configuration is corrent, some validations are necessary for `CronFederatedHPA`, these logic should be implemented in `karmada-webhook`:
* If `spec.scaleTargetRef.apiVersion` is `autoscaling.karmada.io/v1alpha1`, `spec.scaleTargetRef.kind` can only be `FederatedHPA`, `spec.rules[*].targetMinReplicas` and `spec.rules[*].targetMaxReplicas` cannot be empty at the same time.
* If `spec.scaleTargetRef.apiVersion` is not `autoscaling.karmada.io/v1alpha1`, `spec.rules[*].targetReplicas` cannot be empty.
* `spec.rules[*].schedule` should be a valid cron format.
* `maxDelaySeconds` should be smaller than the period interval.

### Story Solution

#### Story 1

Requirements:
* Scale up workloads to 1000 every day at 08:30 AM.
* Scale down workloads to 1 every day at 11:00 AM.


```yaml
apiVersion: autoscaling.karmada.io/v1alpha1
kind: CronFederatedHPA
metadata:
  name: cron-federated-hpa
spec:
  scaleTargetRef:
    apiVersion: app/v1
    kind: Deployment
    name: shop
  rules:
  - ruleName: "Scale-Up"
    schedule: "30 08 * * *"
    targetReplicas: 1000
  - ruleName: "Scale-Down"
    schedule: "0 11 * * *"
    targetReplicas: 1
```

Also, if we want a better approach, we can choose to set `FederatedHPA`'s `MinReplicas`:
```yaml
apiVersion: autoscaling.karmada.io/v1alpha1
kind: CronFederatedHPA
metadata:
  name: cron-federated-hpa
spec:
  scaleTargetRef:
    apiVersion: autoscaling.karmada.io/v1alpha1
    kind: FederatedHPA
    name: shop-fhpa
  rules:
  - ruleName: "Scale-Up"
    schedule: "30 08 * * *"
    targetMinReplicas: 1000
  - ruleName: "Scale-Down"
    schedule: "0 11 * * *"
    targetMinReplicas: 1
```
By using this approach, workloads will not be abruptly scaled down at 11:00 AM, which could potentially result in service interruptions.

#### More

As a cluster administrator, there are international business that targets the United States and China. There will be sharp load peaks when 8:00 AM everyday in the Asia/Shanghai time zone and 8:00 AM everyday in the America/Los_Angeles time zone.
The workloads are deployed in the clusters in china, which are in the Asia/Shanghai time zone. So I want to configure the workloads conveniently to be scaled up. Specifically, I can configure the specific time zone for different cron scaling rules.

Requirements:
* Scale up workloads to 1000 every day at 07:30 AM everyday in the Asia/Shanghai time zone.
* Scale up workloads to 1000 every day at 07:30 AM everyday in the America/Los_Angeles time zone.

```yaml
apiVersion: autoscaling.karmada.io/v1alpha1
kind: CronFederatedHPA
metadata:
  name: cron-federated-hpa
spec:
  scaleTargetRef:
    apiVersion: autoscaling.karmada.io/v1alpha1
    kind: FederatedHPA
    name: shop-fhpa
  rules:
  - ruleName: "scale-up-asia-shanghai"
    schedule: "30 07 * * *"
    targetMinReplicas: 1000
    timeZone: "Asia/Shanghai"
  - ruleName: "scale-up-america-los-angeles"
    schedule: "30 07 * * *"
    targetMinReplicas: 1000
    timeZone: "America/Los_Angeles"
```
With this configuration, the workloads will be scaled up to 1000 replicas at least every day at 07:30 AM in the Asia/Shanghai and America/Los_Angeles time zone.

### High Availability
To maintain high availability for `CronFederatedHPA`, the Karmada control plane should be deployed in a highly available manner, such as across multiple zones. If the Karmada control plane fully goes down, workloads can no longer be scaled.

## Development Plan
This feature is should be implement in three stages:
1. Implement `CronFederatedHPA` controller in `karmada-controller-manager` to scale the workloads or set `FederatedHPA`.
1. Implement the validations in `karmada-webhook`.

## Test Plan
1. All current testing should be passed, no break change would be involved by this feature.
2. Add new E2E test cases to cover the new feature.
   1. Scale workload directly with different time.
   2. Set `FederatedHPA`'s `MinReplicas/MaxReplicas` with different time.
   3. Construct different time and check the validations.

## Alternatives  
