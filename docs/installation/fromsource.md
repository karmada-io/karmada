<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Installing Karmada on Cluster from Source](#installing-karmada-on-cluster-from-source)
  - [Select a way to expose karmada-apiserver](#select-a-way-to-expose-karmada-apiserver)
    - [1. expose by service with `LoadBalancer` type](#1-expose-by-service-with-loadbalancer-type)
    - [2. expose by service with `ClusterIP` type](#2-expose-by-service-with-clusterip-type)
  - [Install](#install)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Installing Karmada on Cluster from Source

This document describes how you can use the `hack/remote-up-karmada.sh` script to install Karmada on 
your clusters based on the codebase.

## Select a way to expose karmada-apiserver

The `hack/remote-up-karmada.sh` will install `karmada-apiserver` and provide two ways to expose the server:

### 1. expose by service with `LoadBalancer` type

By default, the `hack/remote-up-karmada.sh` will expose `karmada-apiserver` by a service with `LoadBalancer`
type, and continue the installation progress after the `Load Balancer` allocates an external IP for the
`karmada-apiserver`.

No extra operations needed with this type, but that *requires your cluster have deployed the `Load Balancer`*.

### 2. expose by service with `ClusterIP` type
If you don't want to use the `Load Balancer`, you can ask `hack/remote-up-karmada.sh` to expose `karmada-apiserver`
by a service with `ClusterIP` type. All you need to do is set an environment:
```bash
export CLUSTER_IP_ONLY=true
```

> Note: You should run `hack/remote-up-karmada.sh` on one of the nodes of the cluster, otherwise the `hack/remote-up-karmada.sh`
> can't access the `karmada-apiserver` exposed by a `cluster IP`.

## Install
From the `root` directory the `karmada` repo, install Karmada by command:
```bash
hack/remote-up-karmada.sh <kubeconfig> <context_name>
```
- `kubeconfig` is your cluster's kubeconfig that you want to install to
- `context_name` is the name of context in 'kubeconfig'

For example:
```bash
hack/remote-up-karmada.sh $HOME/.kube/config mycluster
```

If everything goes well, at the end of the script output, you will see similar messages as follows:
```
Karmada is installed.

Kubeconfig for karmada in file: /root/.kube/karmada.config, so you can run:
  export KUBECONFIG="/root/.kube/karmada.config"
Or use kubectl with --kubeconfig=/root/.kube/karmada.config
Please use 'kubectl config use-context karmada-apiserver' to switch the cluster of karmada control plane
And use 'kubectl config use-context your-host' for debugging karmada installation
```