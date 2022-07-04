<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Configure Controllers](#configure-controllers)
  - [Karmada Controllers](#karmada-controllers)
    - [Configure Karmada Controllers](#configure-karmada-controllers)
  - [Kubernetes Controllers](#kubernetes-controllers)
    - [Required Controllers](#required-controllers)
      - [namespace](#namespace)
      - [garbagecollector](#garbagecollector)
      - [serviceaccount-token](#serviceaccount-token)
    - [Optinal Controllers](#optinal-controllers)
      - [ttl-after-finished](#ttl-after-finished)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Configure Controllers

Karmada maintains a bunch of controllers which are control loops that watch the state of your system, then make or
request changes where needed. Each controller tries to move the current state closer to the desired state.
See [Kubernetes Controller Concepts][1] for more details.

## Karmada Controllers

The controllers are embedded into components of `karmada-controller-manager` or `karmada-agent` and will be launched  
along with components startup. Some controllers may be shared by `karmada-controller-manager` and `karmada-agent`.

| Controller    | In karmada-controller-manager | In karmada-agent |
|---------------|-------------------------------|------------------|
| cluster       | Y                             | N                |
| clusterStatus | Y                             | Y                |
| binding       | Y                             | N                |
| execution     | Y                             | Y                |
| workStatus    | Y                             | Y                |
| namespace     | Y                             | N                |
| serviceExport | Y                             | Y                |
| endpointSlice | Y                             | N                |
| serviceImport | Y                             | N                |
| unifiedAuth   | Y                             | N                |
| hpa           | Y                             | N                |

### Configure Karmada Controllers

You can use `--controllers` flag to specify the enabled controller list for `karmada-controller-manager` and
`karmada-agent`, or disable some of them in addition to the default list.

E.g. Specify a controller list:
```bash
--controllers=cluster,clusterStatus,binding,xxx
```

E.g. Disable some controllers(remember to keep `*` if you want to keep the rest controllers in the default list):
```bash
--controllers=-hpa,-unifiedAuth,*
```
Use `-foo` to disable the controller named `foo`.

> Note: The default controller list might be changed in the future releases. The controllers enabled in the last release
> might be disabled or deprecated and new controllers might be introduced too. Users who are using this flag should
> check the release notes before system upgrade.

## Kubernetes Controllers

In addition to the controllers that are maintained by the Karmada community, Karmada also requires some controllers from
Kubernetes. These controllers run as part of `kube-controller-manager` and are maintained by the Kubernetes community.

Users are recommended to deploy the `kube-controller-manager` along with Karmada components. And the installation
methods list in [installation guide][2] would help you deploy it as well as Karmada components.

### Required Controllers

Not all controllers in `kube-controller-manager` are necessary for Karmada, if you are deploying
Karmada using other tools, you might have to configure the controllers by `--controllers` flag just like what we did in
[example of kube-controller-manager deployment][3].

The following controllers are tested and recommended by Karmada.

#### namespace

The `namespace` controller runs as part of `kube-controller-manager`. It watches `Namespace` deletion and deletes 
all resources in the given namespace.

For the Karmada control plane, we inherit this behavior to keep a consistent user experience. More than that, we also
rely on this feature in the implementation of Karmada controllers, for example, when un-registering a cluster,
Karmada would delete the `execution namespace`(named `karmada-es-<cluster name>`) that stores all the resources 
propagated to that cluster, to ensure all the resources could be cleaned up from both the Karmada control plane and the 
given cluster.

More details about the `namespace` controller, please refer to 
[namespace controller sync logic](https://github.com/kubernetes/kubernetes/blob/v1.23.4/pkg/controller/namespace/deletion/namespaced_resources_deleter.go#L82-L94).

#### garbagecollector

The `garbagecollector` controller runs as part of `kube-controller-manager`. It is used to clean up garbage resources.
It manages [owner reference](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) and
deletes the resources once all owners are absent.

For the Karmada control plane, we also use `owner reference` to link objects to each other. For example, each
`ResourceBinding` has an owner reference that link to the `resource template`. Once the `resource template` is removed,
the `ResourceBinding` will be removed by `garbagecollector` controller automatically.

For more details about garbage collection mechanisms, please refer to
[Garbage Collection](https://kubernetes.io/docs/concepts/architecture/garbage-collection/).

#### serviceaccount-token

The `serviceaccount-token` controller runs as part of `kube-controller-manager`.
It watches `ServiceAccount` creation and creates a corresponding ServiceAccount token Secret to allow API access.

For the Karmada control plane, after a `ServiceAccount` object is created by the administrator, we also need
`serviceaccount-token` controller to generate the ServiceAccount token `Secret`, which will be a relief for
administrator as he/she doesn't need to manually prepare the token.

More details please refer to:
- [service account token controller](https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#token-controller)
- [service account tokens](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#service-account-tokens)

### Optinal Controllers

#### ttl-after-finished

The `ttl-after-finished` controller runs as part of `kube-controller-manager`.
It watches `Job` updates and limits the lifetime of finished `Jobs`. 
The TTL timer starts when the Job finishes, and the finished Job will be cleaned up after the TTL expires.

For the Karmada control plane, we also provide the capability to clean up finished `Jobs` automatically by 
specifying the `.spec.ttlSecondsAfterFinished` field of a Job, which will be a relief for the control plane.

More details please refer to:
- [ttl after finished controller](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/#ttl-after-finished-controller)
- [clean up finished jobs automatically](https://kubernetes.io/docs/concepts/workloads/controllers/job/#clean-up-finished-jobs-automatically)

[1]: https://kubernetes.io/docs/concepts/architecture/controller/
[2]: https://github.com/karmada-io/karmada/blob/master/docs/installation/installation.md
[3]: https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/kube-controller-manager.yaml
