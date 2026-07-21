---
title: Enhancing cluster proxy with member clusters awareness
authors:
- "@jwcesign"
- "@XiShanYongYe-Chang"
reviewers:
- "TBD"
- "TBD"
- "TBD"
approvers:
- "TBD"

creation-date: 2023-07-17

---

# Enhancing cluster proxy with member clusters awareness

<!--
This is the title of your KEP. Keep it short, simple, and descriptive. A good
title can help communicate what the KEP is and should be considered as part of
any review.
-->

## Summary

<!--
This section is incredibly important for producing high-quality, user-focused
documentation such as release notes or a development roadmap. 

A good summary is probably at least a paragraph in length.
-->

In multi-cluster business scenarios, the unified query of resources across multiple clusters is a common requirement. Currently, users can query resources from member clusters using the cluster proxy. If they are using `karmadactl`, they can use the following command:

```sh
karmadactl get deployment coredns -nkube-system
```
This will display information about the `coredns` deployment in each member cluster.  

If users prefer to use `kubectl`, they need to append the URL `/apis/cluster.karmada.io/v1alpha1/clusters/{clustername}/proxy` to their `kubeconfig` file. Here is an example of how it should look in YAML format:

```yaml
apiVersion: v1
clusters:
- cluster:
    # Users will configure {clustername} to query from specific cluster
    server: https://172.18.0.5:5443/apis/cluster.karmada.io/v1alpha1/clusters/{clustername}/proxy
  name: karmada-apiserver
...
```

Once this is done, users can query specific cluster's resources by running the following command:

```sh
kubectl get deployment coredns -nkube-system
```

This proposal propose an enhancement to cluster proxy, to query resources from member clusters using native `kubectl`, without needing to be aware of the individual member clusters.

## Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->

The single cluster users usually use `kubectl` to query resources from member clusters separately. So after migrating to Karmada with multiple clusters, it will be great if they still can use `kubectl` to query resources from member clusters and unaware the member clusters, maintain the same experience as when in a cluster.

Also, Karmada is usually used in multi-tenant environment, user A cloud not access to the resource of user B. This is always achieved by using RBAC. Which means the query operation should follow the RBAC privilege limitations in member clusters.

So this proposal proposes an enhancement to query resources from member clusters with native `kubectl`, unaware the member clusters, and follow RBAC privilege limitations in member clusters.

With this enhancement, users can feel the minimal changes after migrating to Karmada, but enjoy the benefits of multiple clusters.

### Goals

- Propose the idea to query resources from multiple clusters with native `kubectl` and without needing to be aware of the individual member clusters.
- Propose the implementation ideas for involved components.

<!--
List the specific goals of the KEP. What is it trying to achieve? How will we
know that this has succeeded?
-->

### Non-Goals

<!--
What is out of scope for this KEP? Listing non-goals helps to focus discussion
and make progress.
-->

## Proposal

<!--
This is where we get down to the specifics of what the proposal actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real
nitty-gritty.
-->

### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story 1
As a user, I want to migrate my workloads to multiple clusters using Karmada while experiencing minimal differences in the user interface. Specifically, I expect the following:
- The ability to perform operations and maintenance tasks with `kubectl`, just like before.
- No need to be aware of the member clusters.
Therefore, I would like Karmada to provide a seamless and native `kubectl` experience.

#### Story 2
As a platform administrator, I provide a multi-cluster platform by using Karmada, and this platform is used by multiple users. So in order to limit the access range for different users, I configure the RBAC in different member clusters to achieve this. So I want that any operations from the users should follow the RBAC in member clusters.

### Notes/Constraints/Caveats (Optional)

<!--
What are the caveats to the proposal?
What are some important details that didn't come across above?
Go in to as much detail as necessary here.
This might be a good place to talk about core concepts and how they relate.
-->

- If there are resources with the same name in different member clusters, operations such as `get`, `describe`, `exec`, and `logs` will fail.
- The operation `watch` is only allowed for a specific resource. The resource name should exist in only one cluster and does not support namespace or cluster scope.

### Risks and Mitigations

<!--
What are the risks of this proposal, and how do we mitigate? 

How will security be reviewed, and by whom?

How will UX be reviewed, and by whom?

Consider including folks who also work outside the SIG or subproject.
-->

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

### Architecture
The workflow for `list` operations of cluster proxy:  
<img src="./statics/cluster-proxy-list.png" height="300px">  
1. Query target resource from all the clusters with the the account of `kubeconfig`.
2. Aggregate all the valid responses from the clusters and return to users.


The workflow for `get/describe/exec/logs/watch` operations of cluster proxy:  
<img src="./statics/cluster-proxy-get.png" height="300px">  
1. Query target resource from all the clusters with the account of `kubeconfig`, and find out the only one target cluster.
2. Route the request to the target cluster with the account of `kubeconfig`.

The behavior to handle errors will be as follows:
1. If no cluster find the target resource, return error of `not found`.
2. If no privilege in any cluster, return error of `not found`.
3. For `get/describe/exec/logs/watch` operations, if the target resource exists in multiple clusters, return error of `conflict resource`.

#### Components change

`karmada-aggregated-apiserver` is the only component needs to be modified to with following changes:
* Distinguish the operations, including `list/get/describe/exec/logs/watch`.
* For list operations, it will query the target resources from all the member clusters with impersonated user, which will follow the RBAC in member clusters.
* For `get/describe/exec/logs/watch` operations, it will query the target resources from the target cluster with impersonated user, then route the request to the target cluster with impersonated user.
* For `get/describe/exec/logs/watch` operations, if the target resource exists in multiple clusters, return error of `conflict resource`.
* For the operation is not allowed in one cluster, this cluster will be skipped.
* If no cluster find the target resource, return error of `not found`.
* If no privilege in any cluster, return error of `not found`.


### Story solution
After this implementation, users could query resources with cluster proxy using `kubectl`.

#### 1.Append url `/apis/cluster.karmada.io/v1alpha1/clusters/*/proxy` to `kubeconfig`
```yaml
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://172.18.0.5:5443/apis/cluster.karmada.io/v1alpha1/clusters/*/proxy
  name: karmada-apiserver
...
```

#### 2.Do the operations and maintenance with `kubectl`
```sh
# kubectl get pods -A
NAMESPACE            NAME                                            AGE
kube-system          coredns-787d4945fb-l76w8                        5h27m
kube-system          coredns-787d4945fb-zvrln                        5h27m
kube-system          etcd-member1-control-plane                      5h27m
...
# kubectl get pod coredns-787d4945fb-l76w8 -nkube-system
NAME                       READY   STATUS    RESTARTS   AGE
coredns-787d4945fb-l76w8   1/1     Running   0          5h28m
# kubectl describe pod coredns-787d4945fb-l76w8 -nkube-system
Name:                 coredns-787d4945fb-l76w8
Namespace:            kube-system
Priority:             2000000000
...
# kubectl logs coredns-787d4945fb-l76w8 -nkube-system
[INFO] plugin/ready: Still waiting on: "kubernetes"
...
# kubectl exec coredns-787d4945fb-l76w8 -nkube-system -- ls
...
# kubectl get deployment coredns-nkube-system --watch
...
```

All operations will utilize the `kubeconfig` account and route requests to the target cluster. This ensures that RBAC in member clusters is effective, limiting user privileges.

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

1. All the current testing should be passed, no break change would be involved by this feature.
2. Add new E2E test cases to cover the new feature.
    1. `list` resources from multiple clusters, there are different RBAC privileges in different clusters.
    2. `get/describe/exec/logs/watch` resource from multiple clusters, there are different RBAC privileges in different clusters.
    3. `get/describe/exec/logs/watch` resource from the target cluster with impersonated user, there are same name exists in multiple clusters.

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->

If users are willing to use `karmadactl`, it is a better approach to manage resources across multiple clusters as it offers greater flexibility.
- Support operations `list/get/logs/exec/describe/watch/apply/promote`.
- Support cluster management, including `init/deinit/join/unjoin/register/cordon/uncordon/taint`.

One limitation is the need to be aware of individual member clusters when executing commands.
