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

## Features





## Guides

### Quickstart



### User Guide



### Development Guide



## Community



## Code of Conduct
