---
title: Enhance karmadactl operation and maintenance experience
authors:
- "@hulizhe"
- "@zhzhuang-zju"
reviewers:
- "@RainbowMango"

approvers:
- "@RainbowMango"

creation-date: 2024-07-22
---

# Enhance karmadactl operation and maintenance experience

## Summary

Karmadactl is a CLI (Command Line Interface) tool entirely dedicated to Karmada's multi-cluster scenarios. Currently, karmadactl implements commands such as `get` and `describe` for partial applications in a multi-cluster context. For instance, `karmadactl get` is used to display one or many resources in member clusters, and `karmadactl describe` shows detailed information about a specific resource or group of resources within a member cluster. However, these commands are exclusively focused on resources within member clusters, lacking capabilities to perform CRUD (Create, Read, Update, Delete) operations on Karmada control plane resources.

Furthermore, some kubectl commands have not been adapted to multi-cluster scenarios, such as `create`, `label`, and `edit`. This omission prevents karmadactl from functioning independently of kubectl, necessitating the use of both tools to manage resources comprehensively across multi-cluster environments managed by Karmada.

Therefore, this proposal plans to enhance the functionality of karmadactl in multi-cluster scenarios, aiming to further improve the operational experience of using karmadactl. The main enhancements include:

- Enhancing the capabilities of `karmadactl get`, `karmadactl describe`, and similar commands to parse and interpret Karmada control plane resources.
- Completing the implementation of features for commands such as `karmadactl create`, `karmadactl label`, and `karmadactl edit`.

## Motivation

<!--
This section is for explicitly listing the motivation, goals, and non-goals of
this KEP.  Describe why the change is important and the benefits to users.
-->
Currently, due to incomplete capabilities of karmadactl in multi-cluster scenarios, in actual operations and maintenance, karmadactl is often used in conjunction with kubectl. This leads to issues such as switching between karmadactl and kubectl commands under different usage scenarios, and frequent context switching required by kubectl:

1. Suppose Karmada has a deployment named `foo` that has been dispatched to member clusters member1 and member2.
2. If you wish to inspect the `foo` deployment under the Karmada control plane and in each member cluster,
3. Firstly, you would need to use the kubectl command `kubectl get --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver` to display the `foo` deployment in Karmada control plane.
4. Next, you would switch to the karmadactl command `karmadactl get` to check the `foo` deployment in the member clusters.

This disjointed process also creates a fragmented experience for the user, as described in the [issue](https://github.com/karmada-io/karmada/issues/5205).

If karmadactl were capable of viewing resources from Karmada control plane, then the above requirements could be fulfilled simply by using the command `karmadactl get`.

### Goals

- Providing the ability for karmadactl to perform Create, Read, Update, and Delete (CRUD) operations on Karmada control plane resources.
- Bridging the gap by incorporating functionalities into karmadactl that have yet to be inherited from kubectl, notably including `karmadactl create`, `karmadactl delete`, `karmadactl label`, `karmadactl edit`, `karamdactl patch`, `karmadactl annotate`, `karmadctl attach`, `karmadactl api-resources`, `karmadactl explain`, `karmadactl top node`.

### Non-Goals

- Optimization of command output for improved display and readability.
- Replacing kubectl with karmadactl in the [karmada repository](https://github.com/karmada-io/karmada).
- Mining the multi-cluster specific capabilities with karmadactl.

## Proposal

The operation scope for karmadactl commands, based on their specific functionalities, can be categorized as follows:

- Karmada Control Plane
- Karmada Control Plane or a Single Member Cluster
- Karmada Control Plane and Multiple Member Clusters

**To unify the representation of different commands with varying scopes of usage, the following conventions are agreed upon**:

- Default Scope: The default scope for commands is the Karmada Control Plane.
- Specifying Member Clusters: If a command is intended to affect member clusters, the user could specify the target member cluster(s) via a flag. This allows the command to be directed towards a particular member cluster or a list of member clusters.

These guidelines ensure that there is a clear and consistent way to differentiate between commands that operate at the level of the Karamda control plane versus those that target individual or multiple member clusters. This helps prevent ambiguity and ensures that users can accurately predict the behavior of each command based on its scope and the flags provided, facilitating more efficient and error-free operations within the Karmada multi-cluster environment.

Currently, the flag set of karmadactl is only sufficient for viewing or modifying resources within member clusters. Therefore, the first step is to design and modify the flag set of karmadactl, ensuring it can accommodate the various scenarios outlined previously.

Secondly, karmadactl commands can be classified based on whether they inherit from kubectl:

- Inherited from kubectl: These commands represent extensions of single-cluster capabilities to multi-cluster scenarios.
- Multi-cluster-specific abilities: These are commands unique to karmadactl, designed to address the specific requirements and complexities inherent to managing resources across multiple clusters.

To facilitate a smooth transition for users moving from kubectl to karmadactl, efforts should be made to ensure that the user experience for identical commands remains as consistent as possible between the two tools. This includes maintaining similar command structures, output formatting, and behavior wherever feasible, while also leveraging the extended capabilities of karmadactl to manage resources effectively in a multi-cluster environment.

Given that directly modifying resources in member clusters could potentially lead to conflicts between Karmada control plane and the member clusters, karmadactl should not offer the capability to directly create, delete, or update resources in member clusters.

### User Stories (Optional)

<!--
Detail the things that people will be able to do if this KEP is implemented.
Include as much detail as possible so that people can understand the "how" of
the system. The goal here is to make this feel real for users without getting
bogged down.
-->

#### Story 1

As an operations personnel, I hope to be able to view resources from the current Karmada control plane as well as member clusters without context switching or changing between CLIs.

**Scenario**:

1. Given that both the Karmada control plane and member clusters have a deployment named `foo`.
2. When I use the command `karmadactl get`, it will display deployment `foo` from the Karmada control plane as well as from all member clusters, providing a consolidated view of the resource across the multi-cluster environment.

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

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

### CLI flags changes

This proposal proposes new flags `--operation-scope` in karmadactl flag set.

| name            | type   | shorthand | default | usage                                          |
|-----------------|--------|-----------|---------|------------------------------------------------|
| operation-scope | string | /         | karmada | Used to control the operation scope of command |

The value of `operation-scope` is an enumeration, including `karmada`, `members`, and `all`. With the flag:

- The default operation scope for the command is Karmada control plane, and resources in member clusters will be ignored.
- If the desired operation scope for the command is the member clusters, we can set the flag `--operation-scope` to `members`, in which case resources in the Karmada control plane will be ignored.
- If the desired operation scope for the command includes both the karmada control plane and member clusters, we can set the flag `--operation-scope` to `all`.

NOTE: Not all commands include `all` as a value for the `operation-scope` flag; some commands, such as `describe`, can only operate on a single cluster, so `all` will not be included as a value for the `operation-scope` flag in these cases.

#### Interacts with other flags

The `clusters/cluster` flag only takes effect when the `operation-scope` flag is set to `members` or `all`.

### Modification of Existing Commands

#### karmadactl get

- description: Display one or many resources in Karmada control plane and member clusters.
- operation scope: Karmada control plane and member clusters.
- scope-related flag set:

| name            | type     | optional values       | default | usage                                          |
|-----------------|----------|-----------------------|---------|------------------------------------------------|
| clusters        | []string | /                     | none    | Used to specify target member clusters         |
| operation-scope | string   | karmada; members; all | karmada | Used to control the operation scope of command |

- performance:
<ol>
<li>Use the command `karmadactl get` or `karmadactl get --operation-scope karmada` to display resources in the karmada control plane.</li>
<li>Use the command `karmadactl get --operation-scope members` to display resources in all member clusters.</li>
<li>Use the command `karmadactl get --operation-scope members --clusters member1,member2` to display resources in member1 cluster and member2 cluster.</li>
<li>Use the command `karmadactl get --operation-scope all` to display resources in Karmada control plane and all member clusters.</li>
<li>Use the command `karmadactl get --operation-scope all --clusters member1,member2` to display resources in Karmada control plane, member1 cluster and member2 cluster.</li>
</ol>

#### karmadactl describe

- description: Show details of a specific resource or group of resources in Karmada control plane or a member cluster.
- usage scope: Karmada Control Plane or a Single Member Cluster.
- scope-related flag set:

| name            | type   | optional values  | default | usage                                          |
|-----------------|--------|------------------|---------|------------------------------------------------|
| cluster         | string | /                | none    | Used to specify target member cluster          |
| operation-scope | string | karmada; members | karmada | Used to control the operation scope of command |

- performance:
<ol>
<li>Use the command `karmadactl describe` or `karmadactl describe --operation-scope karmada` to show details of resources in the karmada control plane.</li>
<li>Use the command `karmadactl describe --operation-scope members --cluster member1` to show details of resources in the member1 cluster.</li>
<li>When the `operation-scope` flag is set to members and the `cluster` flag is empty, a message should prompt the user with "must specify a cluster".</li>
</ol>


#### karmadactl top pod

- description: Display resource (CPU/memory) usage of pods in Karmada control plane and member clusters..
- usage scope: Karmada control plane and member clusters.
- scope-related flag set:

| name            | type     | optional values       | default | usage                                          |
|-----------------|----------|-----------------------|---------|------------------------------------------------|
| clusters        | []string | /                     | none    | Used to specify target member clusters         |
| operation-scope | string   | karmada; members; all | karmada | Used to control the operation scope of command |

- performance:

<ol>
<li>Use the command `karmadactl top pod` or `karmadactl top pod --operation-scope karmada` to display resource (CPU/memory) usage of pods in Karmada control plane.</li>
<li>Use the command `karmadactl top pod --operation-scope members` to display resource (CPU/memory) usage of pods in all member clusters.</li>
<li>Use the command `karmadactl top pod --operation-scope members --clusters member1,member2` to display resource (CPU/memory) usage of pods in member1 cluster and member2 cluster.</li>
<li>Use the command `karmadactl top pod --operation-scope all` to display resource (CPU/memory) usage of pods in Karmada control plane and all member clusters.</li>
<li>Use the command `karmadactl top pod --operation-scope all --clusters member1,member2` to display resource (CPU/memory) usage of pods in Karmada control plane, member1 cluster and member2 cluster.</li>
</ol>

### Adding New Commands

#### karmadactl edit, create, delete, label, patch and annotate

These commands provide the capability for karmadactl to create, delete, and update resources on the Karmada control plane with functionalities that are consistent with those available in kubectl.

- usage scope: Karmada control plane
- scope-related flag set: None.

#### karmadactl top node

- description: Display resource (CPU/memory) usage of nodes in Karmada control plane and member clusters.
- usage scope: Karmada control plane and member clusters.
- scope-related flag set:

| name            | type     | optional values       | default | usage                                          |
|-----------------|----------|-----------------------|---------|------------------------------------------------|
| clusters        | []string | /                     | none    | Used to specify target member clusters         |
| operation-scope | string   | karmada; members; all | karmada | Used to control the operation scope of command |

- performance:

<ol>
<li>Use the command `karmadactl top node` or `karmadactl top node --operation-scope karmada` to display resource (CPU/memory) usage of nodes in Karmada control plane.</li>
<li>Use the command `karmadactl top node --operation-scope members` to display resource (CPU/memory) usage of nodes in all member clusters.</li>
<li>Use the command `karmadactl top node --operation-scope members --clusters member1,member2` to display resource (CPU/memory) usage of nodes in member1 cluster and member2 cluster.</li>
<li>Use the command `karmadactl top node --operation-scope all` to display resource (CPU/memory) usage of nodes in Karmada control plane and all member clusters.</li>
<li>Use the command `karmadactl top node --operation-scope all --clusters member1,member2` to display resource (CPU/memory) usage of nodes in Karmada control plane, member1 cluster and member2 cluster.</li>
</ol>

#### karmadactl attach
- description: attach to a process that is already running inside an existing container in Karmada control plane or a member cluster.
- usage scope: Karmada Control Plane or a Single Member Cluster.
- scope-related flag set:

| name            | type   | optional values  | default | usage                                          |
|-----------------|--------|------------------|---------|------------------------------------------------|
| cluster         | string | /                | none    | Used to specify target member cluster          |
| operation-scope | string | karmada; members | karmada | Used to control the operation scope of command |

- performance:
<ol>
<li>Use the command `karmadactl attach` or `karmadactl attach --operation-scope karmada` to attach to a process running inside an existing container in the karmada control plane.</li>
<li>Use the command `karmadactl attach --operation-scope members --cluster member1` to attach to a process running inside an existing container in the member1 cluster.</li>
<li>When the `operation-scope` flag is set to members and the `cluster` flag is empty, a message should prompt the user with "must specify a cluster".</li>
</ol>

#### karmadactl api-resources
- description: Print the supported API resources on the server in Karmada control plane or a member cluster.
- usage scope: Karmada Control Plane or a Single Member Cluster
- scope-related flag set:

| name            | type   | optional values  | default | usage                                          |
|-----------------|--------|------------------|---------|------------------------------------------------|
| cluster         | string | /                | none    | Used to specify target member cluster          |
| operation-scope | string | karmada; members | karmada | Used to control the operation scope of command |

- performance:
<ol>
<li>Use the command `karmadactl api-resources` or `karmadactl api-resources --operation-scope karmada` to print the supported API resources on the server in Karmada control plane.</li>
<li>Use the command `karmadactl api-resources --operation-scope members --cluster member1` to attach to print the supported API resources on the server in the member1 cluster.</li>
<li>When the `operation-scope` flag is set to members and the `cluster` flag is empty, a message should prompt the user with "must specify a cluster".</li>
</ol>

#### karmadactl explain
- description: Describe fields and structure of various resources in Karmada control plane or a member cluster.
- usage scope: Karmada Control Plane or a Single Member Cluster
- scope-related flag set:

| name            | type   | optional values  | default | usage                                          |
|-----------------|--------|------------------|---------|------------------------------------------------|
| cluster         | string | /                | none    | Used to specify target member cluster          |
| operation-scope | string | karmada; members | karmada | Used to control the operation scope of command |

- performance:
<ol>
<li>Use the command `karmadactl explain` or `karmadactl explain --operation-scope karmada` to describe fields and structure of various resources in Karmada control plane.</li>
<li>Use the command `karmadactl explain --operation-scope members --cluster member1` to describe fields and structure of various resources in the member1 cluster.</li>
<li>When the `operation-scope` flag is set to members and the `cluster` flag is empty, a message should prompt the user with "must specify a cluster".</li>
</ol>

### Q&A

#### Why choose to complete the above command?

Choosing to complete the above command is based on the following reasons:

- It's a popular command in kubectl.
- There is considerable usage value, which can improve the operations experience.

#### karmadactl hopes to complement the capabilities of kubectl. How should it handle the evolution of kubectl in the future, and will there be significant maintenance costs?

- For commands that are completely consistent with kubectl's capabilities, karmadactl can achieve this by encapsulating kubectl, ensuring its capabilities remain consistent with those of `k8s.io/kubectl` that karmada depends on, eliminating the need for additional maintenance.
- For commands that have evolved from kubectl, they are effectively different commands and can evolve independently. New capabilities added to kubectl in the future can be considered for separate introduction based on demand.
- For new commands added to kubectl, they can be considered for separate introduction based on need.

In summary, the overall maintenance cost is controllable.

#### For commands that are currently identical to kubectl's capabilities, such as `karmadactl delete`, if optimizations are identified during later evolution, how should they be handled?

This situation is likely to occur because many commands specific to Karmada's functionality are yet to be fully explored. Once the requirements become clear, the command can be evolved accordingly. This can be planned as part of the next iteration of karmadactl's optimization.

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
