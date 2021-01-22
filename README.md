# Karmada

Karmada (Kubernetes Armada) is a multi-cluster management system speaks Kubernetes native API.
Karmada aims to provide turnkey automation for multi-cluster application management in multi-cloud and hybrid cloud scenarios, and intended to realize multi-cloud centralized management, high availability, failure recovery and traffic scheduling.

Its key capabilities include:

- Cross-cluster applications managements based on K8s native API, allow user migrate apps from single cluser to multi-cluster conveniently and quickly.
- Support provisioning or attaching Kubernetes clusters for centralized operations and management.
- Cross-cluster applications auto-scaling, failover and load-balancing on multi-cluster.
- Advanced scheduling strategy: region, available zone, cloud provider, cluster affinity/anti-affinity.


**Notice: this project is developed in continuation of Kubernetes [Federation v1](https://github.com/kubernetes-retired/federation) and [v2](https://github.com/kubernetes-sigs/kubefed). Some basic concepts are inherited from these two versions.**


## Architecture

![Architecture](docs/images/architecture.png)

The Karmada Control Plane consists of the following components:

- ETCD for storing the karmada API objects
- Karmada API Server
- Karmada Controller Manager
- Karmada Scheduler

ETCD stores the karmada API objects, the API Server is the REST endpoint all other components talk to, and the Karmada Controller Manager perform operations based on the API objects you create through the API server.

The Karmada Controller Manager runs the various controllers,  the controllers watch karmada objects and then talk to the underlying clusters’ API servers to create regular Kubernetes resources.

1. Cluster Controller: attach kubernetes clusters to Karmada for managing the lifecycle of the clusters by creating cluster object.

2. Policy Controller: the controller watches PropagationPolicy objects. When PropagationPolicy object is added, it selects a group of resources matching the resourceSelector and create PropagationBinding with each single resource object.
3. Binding Controller: the controller watches PropagationBinding object and create PropagationWork object corresponding to each cluster with single resource manifest.
4. Execution Controller: the controller watches PropagationWork objects.When PropagationWork objects are created, it will distribute the resources to member clusters.


## Concepts

**Resource template**: Karmada uses Kubernetes Native API definition for federated resource template, to make it easy to integrate with existing tools that already adopt on Kubernetes

**Propagation Policy**: Karmada offers standalone (placement) policy API to define spreading requirements.
- Support 1:n mapping of Policy: workload, users don't need to indicate scheduling constraints every time creating federated applications.
- With default policies, users can just interact with K8s API

**Override Policy**: Karmada provides standalone override API for specializing cluster relevant configuration automation. E.g.:
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


#### 4. Verify the karmada components:

Now, we are going to checking karmada control plane components running status on `host cluster`, make sure set the 
`KUBECONFIG` environment variable with `host cluster` config file:
```
# export KUBECONFIG="/root/.kube/karmada-host.config"
``` 

The karmada control plane components are expecting to be installed in `karmada-system` namespaces: 
```
# kubectl get deployments.apps -n karmada-system 
NAME                              READY   UP-TO-DATE   AVAILABLE   AGE
karmada-apiserver                 1/1     1            1           16m
karmada-controller-manager        1/1     1            1           14m
karmada-kube-controller-manager   1/1     1            1           16m
```
(Note: `karmada-scheduler` is still under development.)

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
# karmadactl join member1 --member-cluster-kubeconfig=/root/.kube/member1.config
```
The `karmadactl join` command will create a `Cluster` object to reflect the member cluster.

### 3. Check member cluster status
Now, check the member clusters from karmada control plane by following command:
```
# kubectl get cluster
NAME      VERSION   READY   AGE
member1   v1.19.1   True    66s
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

#### 3. Verify the nginx is deployed successfully in member cluster
Start another window, set `KUBECONFIG` with the file we specified in `hack/create-cluster.sh` command,
and then check if the deployment exist:
```
# export KUBECONFIG=/root/.kube/member1.config
# kubectl get deployment
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   1/1     1            1           43s
```
