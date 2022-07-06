# Use Submariner to connect the network between Karmada member clusters

This document demonstrates how to use the `Submariner` to connect the network between member clusters.

[Submariner](https://github.com/submariner-io/submariner) flattens the networks between the connected clusters, and enables IP reachability between Pods and Services.

## Install Karmada

### Install Karmada control plane

Following the steps [Install Karmada control plane](https://github.com/karmada-io/karmada#install-karmada-control-plane) in Quick Start, you can get a Karmada. 

### Join member cluster

In the following steps, we are going to create a member cluster and then join the cluster to Karmada control plane.

1. Create member cluster

We are going to create a cluster named `cluster1` and we want the KUBECONFIG file in $HOME/.kube/cluster.config. Run following command:

```shell
hack/create-cluster.sh cluster1 $HOME/.kube/cluster1.config
```

This will create a cluster by kind.

2. Join member cluster to Karmada control plane

Export `KUBECONFIG` and switch to `karmada apiserver`:

```shell
export KUBECONFIG=$HOME/.kube/karmada.config

kubectl config use-context karmada-apiserver 
```

Then, install `karmadactl` command and join the member cluster:

```shell
go install github.com/karmada-io/karmada/cmd/karmadactl

karmadactl join cluster1 --cluster-kubeconfig=$HOME/.kube/cluster1.config
```

In addition to the original member clusters, ensure that at least two member clusters are joined to the Karmada.

In this example, we have joined two member clusters to the Karmada:

```console
# kubectl get clusters
NAME      VERSION   MODE   READY   AGE
cluster1   v1.21.1   Push   True    16s
cluster2   v1.21.1   Push   True    5s
...
```

## Deploy Submariner

We are going to deploy `Submariner` components on the `host cluster` and `member clusters` by using the `subctl` CLI as it's the recommended deployment method according to [Submariner official documentation](https://github.com/submariner-io/submariner/tree/b4625514061c1d85c10432a78ca0ad46e679367a#installation).

`Submariner` uses a central Broker component to facilitate the exchange of metadata information between Gateway Engines deployed in participating clusters. The Broker must be deployed on a single Kubernetes cluster. This cluster’s API server must be reachable by all Kubernetes clusters connected by Submariner, therefore, we deployed it on the karmada-host cluster.

### Install subctl

Please refer to the [SUBCTL Installation](https://submariner.io/operations/deployment/subctl/).

### Use karmada-host as Broker

```shell
subctl deploy-broker --kubeconfig /root/.kube/karmada.config --kubecontext karmada-host
```

### Join cluster1 and cluster2 to the Broker

```shell
subctl join --kubeconfig /root/.kube/cluster1.config broker-info.subm --natt=false
```

```shell
subctl join --kubeconfig /root/.kube/cluster2.config broker-info.subm --natt=false
```

## Connectivity test

Please refer to the [Multi-cluster Service Discovery](https://github.com/karmada-io/karmada/blob/master/docs/multi-cluster-service.md).
