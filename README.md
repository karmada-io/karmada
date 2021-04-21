# Karmada

![Karmada-logo](docs/images/Karmada-logo-horizontal-color.png)

## Karmada: Open, Multi-Cloud, Multi-Cluster Kubernetes Orchestration

Karmada (Kubernetes Armada) is a Kubernetes management system that enables you to run your cloud-native applications across multiple Kubernetes clusters and clouds, with no changes to your applications. By speaking Kubernetes-native APIs and providing advanced scheduling capabilities, Karmada enables truly open, multi-cloud Kubernetes.

Karmada aims to provide turnkey automation for multi-cluster application management in multi-cloud and hybrid cloud scenarios,
with key features such as centralized multi-cloud management, high availability, failure recovery, and traffic scheduling.


## Why Karmada:
- __K8s Native API Compatible__
    - Zero change upgrade, from single-cluster to multi-cluster
    - Seamless integration of existing K8s tool chain

- __Out of the Box__
    - Built-in policy sets for scenarios, including: Active-active, Remote DR, Geo Redundant, etc.
    - Cross-cluster applications auto-scaling, failover and load-balancing on multi-cluster.

- __Avoid Vendor Lock-in__
    - Integration with mainstream cloud providers
    - Automatic allocation, migration across clusters
    - Not tied to proprietary vendor orchestration

- __Centralized Management__
    - Location agnostic cluster management
    - Support clusters in Public cloud, on-prem or edge

- __Fruitful Multi-Cluster Scheduling Policies__
    - Cluster Affinity, Multi Cluster Splitting/Rebalancing,
    - Multi-Dimension HA: Region/AZ/Cluster/Provider

- __Open and Neutral__
    - Jointly initiated by Internet, finance, manufacturing, teleco, cloud providers, etc.
    - Target for open governance with CNCF



**Notice: this project is developed in continuation of Kubernetes [Federation v1](https://github.com/kubernetes-retired/federation) and [v2](https://github.com/kubernetes-sigs/kubefed). Some basic concepts are inherited from these two versions.**


## Architecture

![Architecture](docs/images/architecture.png)

The Karmada Control Plane consists of the following components:

- Karmada API Server
- Karmada Controller Manager
- Karmada Scheduler

ETCD stores the karmada API objects, the API Server is the REST endpoint all other components talk to, and the Karmada Controller Manager perform operations based on the API objects you create through the API server.

The Karmada Controller Manager runs the various controllers,  the controllers watch karmada objects and then talk to the underlying clusters’ API servers to create regular Kubernetes resources.

1. Cluster Controller: attach kubernetes clusters to Karmada for managing the lifecycle of the clusters by creating cluster object.

2. Policy Controller: the controller watches PropagationPolicy objects. When PropagationPolicy object is added, it selects a group of resources matching the resourceSelector and create ResourceBinding with each single resource object.
3. Binding Controller: the controller watches ResourceBinding object and create Work object corresponding to each cluster with single resource manifest.
4. Execution Controller: the controller watches Work objects.When Work objects are created, it will distribute the resources to member clusters.


## Concepts

**Resource template**: Karmada uses Kubernetes Native API definition for federated resource template, to make it easy to integrate with existing tools that already adopt on Kubernetes

**Propagation Policy**: Karmada offers standalone Propagation(placement) Policy API to define multi-cluster scheduling and spreading requirements.
- Support 1:n mapping of Policy: workload, users don't need to indicate scheduling constraints every time creating federated applications.
- With default policies, users can just interact with K8s API

**Override Policy**: Karmada provides standalone Override Policy API for specializing cluster relevant configuration automation. E.g.:
- Override image prefix according to member cluster region
- Override StorageClass according to cloud provider


The following diagram shows how Karmada resources are involved when propagating resources to member clusters.

![karmada-resource-relation](docs/images/karmada-resource-relation.png)

## Quick Start

This guide will cover:
- Install `karmada` control plane components in a Kubernetes cluster which as known as `host cluster`.
- Join a member cluster to `karmada` control plane.
- Propagate an application by `karmada`.

### Prerequisites
- Go version v1.14+
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) version v1.19+
- [kind](https://kind.sigs.k8s.io/) version v0.9.0+

### Install karmada control plane

#### 1. Clone this repo to your machine:
```
# git clone https://github.com/karmada-io/karmada
```

#### 2. Change to karmada directory:
```
# cd karmada
```

#### 3. Deploy and run karmada control plane:
```
# hack/local-up-karmada.sh
```
The script `hack/local-up-karmada.sh` will do following tasks for you:
- Start a Kubernetes cluster to run the karmada control plane, aka. the `host cluster`.
- Build karmada control plane components based on current codebase.
- Deploy karmada control plane components on `host cluster`.

If everything goes well, at the end of the script output, you will see similar messages as follows:
```
Local Karmada is running.
To start using your karmada, run:
  export KUBECONFIG=/var/run/karmada/karmada-apiserver.config
To start checking karmada components running status on the host cluster, please run:
  export KUBECONFIG="/root/.kube/karmada-host.config"
```

The two `KUBECONFIG` files after the script run are:
- karmada-apiserver.config
- karmada-host.config

The `karmada-apiserver.config` is the **main kubeconfig** to be used when interacting with karamda control plane, while `karmada-host.config` is only used for debuging karmada installation with host cluster.

### Join member cluster
In the following steps, we are going to create a member cluster and then join the cluster to 
karmada control plane.

#### 1. Create member cluster
We are going to create a cluster named `member1` and we want the `KUBECONFIG` file 
in `/root/.kube/member1.config`. Run following comand:
```
# hack/create-cluster.sh member1 /root/.kube/member1.config
```
The script `hack/create-cluster.sh` will create a standalone cluster.

#### 2. Join member cluster to karmada control plane
The command `karmadactl` will help to join the member cluster to karmada control plane, 
before that, we should set `KUBECONFIG` to karmada apiserver:
```
# export KUBECONFIG=/var/run/karmada/karmada-apiserver.config
```

Then, install `karmadactl` command and join the member cluster:
```
# go get github.com/karmada-io/karmada/cmd/karmadactl
# karmadactl join member1 --cluster-kubeconfig=/root/.kube/member1.config
```
The `karmadactl join` command will create a `Cluster` object to reflect the member cluster.

### 3. Check member cluster status
Now, check the member clusters from karmada control plane by following command:
```
# kubectl get clusters
NAME      VERSION   MODE   READY   AGE
member1   v1.20.2   Push   True    66s
```

### Propagate application
In the following steps, we are going to propagate a deployment by karmada.

#### 1. Create nginx deployment in karmada.
First, create a [deployment](samples/nginx/deployment.yaml) named `nginx`:
```
# kubectl create -f samples/nginx/deployment.yaml
```

#### 2. Create PropagationPolicy that will propagate nginx to member cluster
Then, we need create a policy to drive the deployment to our member cluster.
```
# kubectl create -f samples/nginx/propagationpolicy.yaml
``` 

#### 3. Check the deployment status from karmada
You can check deployment status from karmadda, don't need to access member cluster:
```
# kubectl get deployment
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   1/1     1            1           43s
```

## Contributing

If you're interested in being a contributor and want to get involved in
developing the Karmada code, please see [CONTRIBUTING](CONTRIBUTING.md) for
details on submitting patches and the contribution workflow.

## License

Karmada is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
