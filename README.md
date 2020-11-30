# Karmada

Karmada is a multi-cluster management system speaks Kubernetes native API. 
Karmada aims to provide turnkey automation for multi-cluster application management in multi-cloud and hybrid cloud scenarios, and intended to realize multi-cloud centralized management, high availability, failure recovery and traffic scheduling.

Its key capabilities include:

- Cross-cluster applications managements based on k8s native API, allow user migrate apps from single cluser to multi-cluster conveniently and quickly.
- Support provisioning or attaching Kubernetes clusters for centralized operations and management.
- Cross-cluster applications auto-scaling ,failover and loadbalancing on multi-cluster.
- Advanced scheduling strategy: region, available zone, cloud provider, cluster affinity/anti-affinity.

----

## Concepts

![Architecture](docs/images/architecture.png)

The Karmada Control Plane consists of the following components:

- ETCD for storing the karmada API objects
- Karmada API server
- Karmada Controller Manager
- Karmada Scheduler

ETCD stores the karmada API objects, the API server is the REST endpoint all other components talk to, and the Karmada Controller Manager perform operations based on the API objects you create through the API server.

The Karmada Controller Manager runs the various controllers,  the controllers watch karmada objects and then talk to the underlying clusters’ API servers to create regular Kubernetes resources.

1. Cluster Controller: attach kubernetes clusters to Karmada for managing the lifecycle of the clusters by creating membercluster object.

2. Policy Controller: the controller watches PropagationPolicy objects. When PropagationPolicy object is added, it selects a group of resources matching the resourceSelector and create PropagationBinding with each single resource object.
3. Binding Controller: the controller watches PropagationBinding object and create PropagationWork object corresponding to each cluster with single resource manifest.
4. Excution Controller: the controller watches PropagationWork objects.When PropagationWork objects are created, it will distribute the resources to member clusters.

The following figure shows how Karmada resource relate to the objects created in the underlying clusters.

![karmada-resource-relation](docs/images/karmada-resource-relation.png)

## Quickstart

Please install [kind](https://kind.sigs.k8s.io/) and [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) before the following steps.

### Install karmada

1. Clone this repo to get karmada.

```
git clone https://github.com/huawei-cloudnative/karmada
```

2. Move to the karmada package directory.

```
cd karmada
```

3. Install karmada.

```
export KUBECONFIG=/root/.kube/karmada.config
hack/local-up-karmada.sh
```

4. Verify that karmada's component is deployed and running.

```
kubectl get pods -n karmada-system
```

### Join member cluster

1. Create **member cluster** for attaching it to karmada.

```
hack/create-cluster.sh member-cluster-1 /root/.kube/membercluster1.config
```

2. Join member cluster to karmada using karmadactl.

```
make karmadactl
./karmadactl join member-cluster-1 --member-cluster-kubeconfig=/root/.kube/membercluster1.config
```

3. Verify member cluster is Joined to karmada successfully.

```
kubectl get membercluster -n karmada-cluster
```

### Propagate application

1. Create nginx deployment in karmada.

```
export KUBECONFIG=/root/.kube/karmada.config
kubectl create -f samples/nginx/deployment.yaml
```

2. Create PropagationPolicy that will propagate nginx to member cluster.

```
export KUBECONFIG=/root/.kube/karmada.config
kubectl create -f samples/nginx/propagationpolicy.yaml
```

3. Verify the nginx is deployed successfully in **member-cluster-1**.

```
export KUBECONFIG=/root/.kube/membercluster1.config
kubectl describe deploy nginx
```
