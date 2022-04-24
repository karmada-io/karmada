# What's New
## Upgrading support
Users are now able to upgrade from the previous version smoothly. With the 
[multiple version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#specify-multiple-versions)
feature of CRD,
objects with different schemas can be automatically converted between versions. Karmada uses the [semantic 
versioning](https://semver.org/) and will provide workarounds for inevitable breaking changes.

In this release, the ResourceBining and ClusterResourceBinding promote to v1alpha2 and the previous v1alpha1 
version is still available for one more release. With the 
[upgrading instruction](https://github.com/karmada-io/karmada/tree/9126cffa218871e921510d3deb1b185c8a4dee3d/docs/upgrading),
the previous version of 
Karmada can promote smoothly.

## Introduced karmada-scheduler-estimator to facilitate end-to-end scheduling accuracy
Karmada scheduler aims to assign workload to clusters according to constraints and available resources of 
each member cluster. The `kube-scheduler` working on each cluster takes the responsibility to assign Pods 
to Nodes.
Even though Karmada has the capacity to reschedule failure workload between member clusters, but the 
community still commits lots of effort to improve the accuracy of the end-to-end scheduling.

The `karmada-scheduler-estimator` is the effective assistant of `karmada-scheduler`, it provides 
prediction-based scheduling decisions that can significantly improve the scheduling efficiency and 
avoid the wave of rescheduling among clusters. Note that this feature is implemented as 
a pluggable add-on. For the instructions please refer to 
[scheduler estimator guideline](https://github.com/karmada-io/karmada/blob/9126cffa218871e921510d3deb1b185c8a4dee3d/docs/scheduler-estimator.md).


## Maintainability improvements
A bunch of significant maintainability improvements were added to this release, including:

Simplified Karmada installation with 
[helm chart](https://github.com/karmada-io/karmada/tree/9126cffa218871e921510d3deb1b185c8a4dee3d/charts).

Provided metrics to observe scheduler status, the metrics API now served at `/metrics` of `karmada-scheduler`.
With these metrics, users are now able to evaluate the scheduler's performance and identify the bottlenecks.

Provided events to Karmada API objects as supplemental information to debug problems.

# Other Notable Changes
- karmada-controller-manager: The ResourceBinding/ClusterResourceBinding won't be deleted after associate 
  PropagationPolicy/ClusterPropagationPolicy is removed and is still available until resource template is
  removed.([#601](https://github.com/karmada-io/karmada/pull/601))
  
- Introduced --leader-elect-resource-namespace which is used to specify the namespace of election object 
  to components karmada-controller-manager/karmada-scheduler/karmada-agent`. 
  ([#698](https://github.com/karmada-io/karmada/pull/698))
  
- Deprecation: The API ReplicaSchedulingPolicy has been deprecated and will be removed from the following 
  release. The feature now has been integrated into ReplicaScheduling.
- Introduced kubectl-karmada commands as the extensions for kubectl.
  ([#686](https://github.com/karmada-io/karmada/pull/686))
  
- karmada-controller-manager introduced a version command to represent version information.
  ([#717](https://github.com/karmada-io/karmada/pull/717) )
  
- karmada-scheduler/karmada-webhook/karmada-agent/karmada-scheduler-estimator introduced a version command to 
  represent version information. ([#719](https://github.com/karmada-io/karmada/pull/719) )
  
- Provided instructions about how to use the Submariner to connect the network between member 
  clusters. ([#737](https://github.com/karmada-io/karmada/pull/737) )
  
- Added four metrics to the karmada-scheduler to monitor scheduler performance. 
  ([#747](https://github.com/karmada-io/karmada/pull/747))
  