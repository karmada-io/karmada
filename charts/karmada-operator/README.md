# karmada-operator

![Version: 0.0.1](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: latest](https://img.shields.io/badge/AppVersion-latest-informational?style=flat-square)

The Karmada operator is a method for installing, upgrading, and deleting Karmada instances. It builds upon the basic Karmada resource and controller concepts and provides convenience to centrally manage the entire lifecycle of Karmada instances in a global cluster. With the operator, you can extend Karmada with custom resources (CRs) to manage your instances not only in local clusters but also in remote clusters.

This page provides instructions on how to install, upgrade, and uninstall Karmada-operator.

## Prerequisites

- Kubernetes 1.16+
- helm v3+

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | common | 1.x.x |

## Installing or upgrading the Chart

Go to the root directory of the `karmada-io/karmada` repo. To install or upgrade the Helm Chart with
the release name `karmada-operator` in the `karmada-system` namespace.

```bash
helm upgrade --install karmada-operator --create-namespace ./charts/karmada-operator \
--set operator.replicaCount=<replica_count> \
--set operator.image.registry=<image_registry> \
--set operator.image.repository=<image_repository> \
--set operator.image.tag=<image_tag>
-n karmada-system --version <chart_version>
```

For example, you can simply run the helm command:

```bash
helm install karmada-operator -n karmada-system --create-namespace --dependency-update ./charts/karmada-operator
```

Then, check the status of `karmada-operator` pod.

```console
kubectl get po -n karmada-system
NAME                               READY   STATUS    RESTARTS   AGE
karmada-operator-df95586f6-d5bll   1/1     Running   0          6s
```

## Uninstalling the Chart

To uninstall/delete the `karmada-operator` helm release in the `karmada-system` namespace, run:

```bash
helm uninstall karmada-operator -n karmada-system
```

The command removes all the Kubernetes components associated with the chart and deletes the release.


## Values

This chart supports the following values:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| global.imagePullSecrets | list | `[]` |  |
| global.imageRegistry | string | `""` |  |
| operator.affinity | object | `{}` |  |
| operator.image.pullPolicy | string | `"IfNotPresent"` |  |
| operator.image.pullSecrets | list | `[]` |  |
| operator.image.registry | string | `"docker.io"` |  |
| operator.image.repository | string | `"karmada/karmada-operator"` |  |
| operator.image.tag | string | `"latest"` |  |
| operator.labels | object | `{}` |  |
| operator.nodeSelector | object | `{}` |  |
| operator.podAnnotations | object | `{}` |  |
| operator.podLabels | object | `{}` |  |
| operator.replicaCount | int | `1` |  |
| operator.resources | object | `{}` |  |
| operator.strategy.rollingUpdate.maxSurge | string | `"50%"` |  |
| operator.strategy.rollingUpdate.maxUnavailable | int | `0` |  |
| operator.strategy.type | string | `"RollingUpdate"` |  |
| operator.tolerations | list | `[]` |  |
