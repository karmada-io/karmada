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

## Creating Karmada Instances
Once the Karmada Operator is installed and running, you can now create Karmada objects

- Prepare a manifest for a Karmada instance like the following:
```yaml
apiVersion: operator.karmada.io/v1alpha1
kind: Karmada
metadata:
  name: tenant1
  namespace: karmada-system
spec:
  components:
    karmadaAPIServer:
      serviceType: ClusterIP
```

- Apply the manifest:
```shell
kubectl apply -f tenant1.yaml
```

-  Use the following command to wait for the Ready condition of the Karmada object:
```shell
kubectl wait --for=condition=Ready karmada/tenant1 -n karmada-system --timeout=300s
```

This command will block until the Karmada object becomes ready or until the timeout (300 seconds) is reached.

Alternatively, you can check the status manually:
```shell
kubectl get karmada tenant1 -n karmada-system -o=jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

If the output is `True`, the `Karmada` instance is ready.

Once the `Karmada` instance has reached `Ready` state, you'll have references to both the admin kubeconfig and the API Server service.
The kubeconfig can be used to access the instance. Info of the API Server service is important in contexts where the `Karmada` instance is owned by a
higher level operator that must perform additional tasks such as setting up ingress traffic once the `Karmada` instead has reached ready state.
Run the following command to inspect the instance:
```shell
 kubectl -n karmada-system describe karmada tenant1 
```

Sample output (trimmed for brevity):
```text
Name:         tenant1
Namespace:    karmada-system
API Version:  operator.karmada.io/v1alpha1
Kind:         Karmada
.
.
.
Spec:
.
.
.
Status:
  API Server Service:
    Name:  tenant1-karmada-apiserver
  Conditions:
    Last Transition Time:  2024-11-27T01:16:26Z
    Message:               karmada init job is completed
    Reason:                Completed
    Status:                True
    Type:                  Ready
  Secret Ref:
    Name:       tenant1-karmada-admin-config
    Namespace:  karmada-system
.
.
.
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
