# Karmada operator

## Overview

The Karmada operator is a method for installing, upgrading, and deleting Karmada instances.
It builds upon the basic Karmada resource and controller concepts, provides convenience to
centrally manage entire lifecycle of Karmada instances in a global cluster. With the operator,
you can extend Karmada with custom resources (CRs) to manage your instances not only in local
clusters but also in remote clusters.

This document is an overview of how the operator works from a user perspective.

## Developer quick start

This section describes how to install `karmada-operator` and create a Karmada instance with CR.

### Prerequisites

- Kubernetes 1.16+
- Helm v3+

### Deploy `karmada-operator`

#### Helm install

Go to the root directory of the `karmada-io/karmada` repo. To install the Helm Chart with
the release name `karmada-operator` in the namespace `karmada-system`, simply run the helm command:

```shell
helm install karmada-operator -n karmada-system  --create-namespace --dependency-update ./charts/karmada-operator --debug
```

#### Using YAML resource

The `karmada-operator` workload requires a kubeconfig of the local cluster to establish a connection with the cluster and watch CR resources.
In preparation for this, create a secret containing the kubeconfig for the karmada-operator.

```shell
kubectl create namespace karmada-system
kubectl create secret generic my-kubeconfig --from-file=$HOME/.kube/config -n karmada-system
```

Deploy the `karmada-operator` workload.

```shell
kubectl apply -f operator/config/deploy/karmada-operator.yaml
```

The pod of `karmada-operator` in the `karmada-system` namespace will be running.

```shell
kubectl get po -n karmada-system
```
```none
NAME                               READY   STATUS    RESTARTS   AGE
karmada-operator-5b7f485c5-g5lj5   1/1     Running   0          26s
```

### Install Karmada operator crds

```shell
kubectl apply -f operator/config/crds/
```

### Create a Karmada instance

The Karmada operator provides a Karmada CR that can define most configurations for Karmada components.
It includes `image` messages, `replica`, the `args` of binary file, and custom `label`, `annotation`, and `featuregate`.
For details, see [API](./pkg/apis/operator/v1alpha1/type.go).

A Karmada CR represents a Karmada instance, which is a namespace-scoped resource.
The example below is to create a simple Karmada CR in the `test` namespace:

```shell
kubectl create namespace test
kubectl apply -f - <<EOF
apiVersion: operator.karmada.io/v1alpha1
kind: Karmada
metadata:
  name: karmada-demo
  namespace: test
EOF
```

Wait for around 40 seconds, and the pods of the Karmada components will be running in the same namespace as the Karmada CR.

```shell
kubectl get po -n test
karmada-demo-aggregated-apiserver-587bc5c697-v27vb      1/1     Running   0          12s
karmada-demo-apiserver-55968d9f8c-mp8hf                 1/1     Running   0          35s
karmada-demo-controller-manager-64455f7fd4-stls6        1/1     Running   0          5s
karmada-demo-etcd-0                                     1/1     Running   0          37s
karmada-demo-kube-controller-manager-584f978bbd-fftwq   1/1     Running   0          5s
karmada-demo-scheduler-6d77b7547-hgz8n                  1/1     Running   0          5s
karmada-demo-webhook-6f5944f5d8-bpkqz                   1/1     Running   0          5s
```

> **Tip**:
>
> If no `spec.hostCluster.secretRef` is specified in CR, the Karmada instance will be installed in the cluster where `karmada-operator` is located.

### Upgrade a Karmada instance

Once a Karmada instance is created, the CR resource is automatically filled with default values.
To upgrade the Karmada instance, for example, you can upgrade the Karmada version to v1.5.0 or higher:

```shell
kubectl patch karmada karmada-demo -n test --type merge -p '
{
  "spec": {
    "components": {
      "karmadaAggregatedAPIServer": {
        "imageTag": "v1.5.0"
      },
      "karmadaControllerManager": {
        "imageTag": "v1.5.0"
      },
      "karmadaScheduler": {
        "imageTag": "v1.5.0"
      },
      "karmadaWebhook": {
        "imageTag": "v1.5.0"
      }
    }
  }
}'
```

### Delete a Karmada instance

Deleting a Karmada CR is a delicate operation that requires careful attention.
Once the Karmada CR is deleted, the associated Karmada instance will also be deleted.
It is important to proceed with caution when deleting a Karmada CR due to the potential risks involved.

```shell
kubectl delete karmada karmada-demo -n test
```

If you want to delete a Karmada CR without cascading deletion of the associated Karmada instance,
you can run the following command before performing the deletion operation.

```shell
kubectl label karmada karmada-demo -n test operator.karmada.io/disable-cascading-deletion=true
```

### Custom Karmada CR

This feature allows you to configure the Karmada CR to install Karmada instances flexibly.
For details, see [karmada.yaml](./config/samples/karmada.yaml).

#### Set Karmada component replicas

The `replicas` of all Karmada components can be modified.
For example, you can scale the etcd pod `replicas` to 3:

```yaml
apiVersion: operator.karmada.io/v1alpha1
kind: Karmada
metadata:
  name: karmada-demo
  namespace: test
spec:
  components:
    etcd:
      local:
        replicas: 3
```

#### Custom label and annotation

All Karmada components allow for custom labels and annotations to be set.
These are merged into both pod and workload resources.

```yaml
apiVersion: operator.karmada.io/v1alpha1
kind: Karmada
metadata:
  name: karmada-demo
  namespace: test
spec:
  components:
    karmadaAPIServer:
      labels: 
        <custom-label-key>: <custom-label-value>
      annotations:
        <custom-annotation-key>: <custom-annotation-value>
```

#### Change karmada-apiserver service type

The service type of karmada-apiserver is `ClusterIP` by default.
You can change it to `NodePort`:

```yaml
...
karmadaAPIServer:
  imageRepository: registry.k8s.io/kube-apiserver
  imageTag: v1.25.4
  replicas: 1
  serviceType: NodePort
  serviceSubnet: 10.96.0.0/12
...
```

#### Add karmada-apiserver SANs

You can add more SANs to karmada-apiserver certificate:

```yaml
...
karmadaAPIServer:
  imageRepository: registry.k8s.io/kube-apiserver
  imageTag: v1.25.4
  replicas: 1
  serviceSubnet: 10.96.0.0/12
  certSANs:
  - "kubernetes.default.svc"
  - "127.0.0.1"
...
```

#### Install karmada addon

By default, the Karmada operator does not install the `descheduler` and `search` addons.
If you want to use them, you should add definitions to the Karmada CR.
Here is an example of the `descheduler` addon:

```yaml
apiVersion: operator.karmada.io/v1alpha1
kind: Karmada
metadata:
  name: karmada-demo
  namespace: test
spec:
  components:
    KarmadaDescheduler: {}
```

If you want to install with the defaults, simply define an empty struct for `descheduler`.

> **Tip**:
>
> Now, we only support installing the `descheduler` addon.

## Contributing

The `karmada/operator` repo is part of Karmada from 1.5 onwards. If you're interested in
the Karmada operator and want to contribute your code and ideas, welcome to open PRs and issues.
See [CONTRIBUTING](../CONTRIBUTING.md) for details on submitting patches and the contribution workflow.
