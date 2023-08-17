---
title: Service discovery with native Kubernetes naming and resolution
authors:
- "@bivas"
reviewers:
- "@XiShanYongYe-Chang"
- TBD
approvers:
- "@robot"
- TBD

creation-date: 2023-06-22

---

# Service discovery with native Kubernetes naming and resolution

## Summary

With the current `ServiceImportController` when a `ServiceImport` object is reconciled, the derived service is prefixed with `derived-` prefix.

## Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->
Having a `derived-` prefix for `Service` resources seems counterintuitive when thinking about service discovery:
- Assuming the pod is exported as the service `foo`
- Another pod that wishes to access it on the same cluster will simply call `foo` and Kubernetes will bind to the correct one
- If that pod is scheduled to another cluster, the original service discovery will fail as there's no service by the name `foo`
- To find the original pod, the other pod is required to know it is in another cluster and use `derived-foo` to work properly

### Goals

- Remove the "derived-" prefix from the service
- User-friendly and native service discovery

### Non-Goals

- Multi cluster connectivity

## Proposal

Following are flows to support the service import proposal:

1. `Deployment` and `Service` are created on cluster member1 and the `Service` imported to cluster member2 using `ServiceImport` (described below as [user story 1](#story-1))
2. `Deployment` and `Service` are created on cluster member1 and both propagated to cluster member2. `Service` from cluster member1 is imported to cluster member2 using `ServiceImport` (described below as [user story 2](#story-2))

The proposal for this flow is what can be referred to as local-and-remote service discovery. The options could be:

1. **Local** only - In case there's a local service by the name `foo` Karmada never attempts to import the remote service and doesn't create an `EndPointSlice`
2. **Local** and **Remote** - Users accessing the `foo` service will reach either member1 or member2
3. **Remote** only - in case there's a local service by the name `foo` Karmada will remove the local `EndPointSlice` and will create an `EndPointSlice` pointing to the other cluster (e.g. instead of resolving member2 cluster is will reach member1)

This proposal suggests to distinguish the services type by the extending the API `multicluster.x-k8s.io/v1alpha1` of `ServiceImport` with Karmada specific annotations:
```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ServiceImport
metadata:
  name: serve
  namespace: demo
  annotations:
    discovery.karmada.io/strategy: LocalAndRemote
spec:
  type: ClusterSetIP
  ports:
  - port: 80
    protocol: TCP
```

The default could be `LocalAndRemote`.


### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story 1

As a Kubernetes cluster member,
I want to access a service from another cluster member,
So that I can communicate with the service using its original name.

**Background**: The Service named `foo` is created on cluster member1 and imported to cluster member2 using `ServiceImport`.

**Scenario**:

1. Given that the `Service` named `foo` exists on cluster member1
2. And the `ServiceImport` resource is created on cluster member2, specifying the import of `foo`
3. When I try to access the service inside member2
4. Then I can access the service using the name `foo.myspace.svc.cluster.local`

#### Story 2

As a Kubernetes cluster member,
I want to handle conflicts when importing a service from another cluster member,
So that I can access the service without collisions and maintain high availability.

**Background**: The Service named `foo` is created on cluster member1 and has a conflict when attempting to import to cluster member2.
Conflict refers to the situation where there is already a `Service` `foo` existing on the cluster (e.g. propagated with `PropagationPolicy`), but we still need to import `Service` `foo` from other clusters onto this cluster (using `ServiceImport`)

**Scenario**:

1. Given that the `Service` named `foo` exists on cluster member1
2. And there is already a conflicting `Service` named foo on cluster member2
3. When I attempt to access the service in cluster member2 using `foo.myspace.svc.cluster.local`
4. Then the requests round-robin between the local `foo` service and the imported `foo` service (member1 and member2)

### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go in to as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate? 

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->
Adding a `Service` that resolve to a remote cluster will add a network latency of communication between clusters. 

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

To achieve service discovery with native Kubernetes naming and resolution, the following design details should be considered:

1. Service Import Controller Changes: The Service Import Controller needs to be updated to remove the `"derived-"` prefix from the derived service. This ensures that the imported service retains its original name when accessed within the importing cluster.
2. Local-Only Service Discovery: To support local-only service discovery, Karmada should be enhanced to check if there is a local service with the same name as the imported service. If a local service exists, the remote service import should be skipped, and no `EndPointSlice` should be created for the remote service. This allows local services to be accessed directly without going through the remote service.
3. Local and Remote Service Discovery: When a service is imported from another cluster, both local and remote services should be accessible. Users accessing the service should be able to reach either the local or remote member based on their location and cluster routing. This enables high availability and load balancing between the local and remote instances of the service.
4. Remote-Only Service Discovery: In cases where there is local service with the same name as the imported service, Karmada should remove the local `EndPointSlice` and create an `EndPointSlice` pointing to the remote cluster. This allows users to access the service through the imported `EndPointSlice`, effectively reaching the remote cluster.

Implementing the service registration flow, the following should be considered:
1. Given that the `Service` named `foo` exists on cluster member1
2. The `ServiceImport` resource is created on cluster member2, specifying the import of `foo`
3. A `Service` resource is created in cluster member2, but does not prefix the name with `derived-`
   1. If there is already an existing `Service` named `foo` on cluster member2
   2. The `EndPointSlice` associated with the local `foo` service points to the local `Deployment` running on member2
   3. The controller will ignore the conflict of existing `Service` and will not create a new `Service` object
4. And the imported `EndPointSlice` is created, pointing to the remote `Deployment` running on member1
5. And the `EndPointSlice` associated with the imported service is prefixed with `import-`
6. Then the requests round-robin between the local `foo` service and the imported `foo` service (member1 and member2)

### Test Plan

<!--
**Note:** *Not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all test cases, just the general strategy. Anything
that would count as tricky in the implementation, and anything particularly
challenging to test, should be called out.

-->

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

One alternative approach to service discovery with native Kubernetes naming and resolution is to rely on external DNS-based service discovery mechanisms. However, this approach may require additional configuration and management overhead, as well as potential inconsistencies between different DNS implementations. By leveraging the native Kubernetes naming and resolution capabilities, the proposed solution simplifies service discovery and provides a consistent user experience.

Another alternative approach could be to enforce a strict naming convention for imported services, where a specific prefix or suffix is added to the service name to differentiate it from local services. However, this approach may introduce complexity for users and require manual handling of naming collisions. The proposed solution aims to provide a more user-friendly experience by removing the "derived-" prefix and allowing services to be accessed using their original names.

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->