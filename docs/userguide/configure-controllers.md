<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Configure Controllers](#configure-controllers)
  - [Karmada Controllers](#karmada-controllers)
    - [Configure Karmada Controllers](#configure-karmada-controllers)
  - [Kubernetes Controllers](#kubernetes-controllers)
    - [Recommended Controllers](#recommended-controllers)
      - [namespace](#namespace)
      - [garbagecollector](#garbagecollector)
      - [serviceaccount-token](#serviceaccount-token)

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

E.g. Disable some controllers:
```bash
--controllers=-hpa,-unifiedAuth
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

### Recommended Controllers

Not all controllers in `kube-controller-manager` are necessary for Karmada, if you are deploying
Karmada using other tools, you might have to configure the controllers by `--controllers` flag just like what we did in
[example of kube-controller-manager deployment][3].

The following controllers are tested and recommended by Karmada.

#### namespace
TODO.

#### garbagecollector
TODO.

#### serviceaccount-token

The `serviceaccount-token` controller runs as part of `kube-controller-manager`.
It watches `ServiceAccount` creation and creates a corresponding ServiceAccount token Secret to allow API access.

For the Karmada control plane, after a `ServiceAccount` object is created by the administrator, we also need
`serviceaccount-token` controller to generate the ServiceAccount token `Secret`, which will be a relief for
administrator as he/she doesn't need to manually prepare the token.

More details please refer to:
- [service account token controller](https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#token-controller)
- [service account tokens](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#service-account-tokens)

[1]: https://kubernetes.io/docs/concepts/architecture/controller/
[2]: https://github.com/karmada-io/karmada/blob/master/docs/installation/installation.md
[3]: https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/kube-controller-manager.yaml
