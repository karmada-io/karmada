# Failover Overview

## Monitor the cluster health status

Karmada supports both `Push` and `Pull` modes to manage member clusters.

More details about cluster registration please refer to [Cluster Registration](./cluster-registration.md#cluster-registration).

### Determining failures

For clusters there are two forms of heartbeats:
- updates to the `.status` of a Cluster.
- `Lease` objects within the `karmada-cluster` namespace in karmada control plane. Each cluster has an associated `Lease` object.

#### Cluster status collection

For `Push` mode clusters, the cluster status controller in karmada control plane will continually collect cluster's status for a configured interval.

For `Pull` mode clusters, the `karmada-agent` is responsible for creating and updating the `.status` of clusters with configured interval.

The interval for `.status` updates to `Cluster` can be configured via `--cluster-status-update-frequency` flag(default is 10 seconds).

Cluster might be set to the `NotReady` state with following conditions:
- cluster is unreachable(retry 4 times within 2 seconds).
- cluster's health endpoint responded without ok.
- failed to collect cluster status including the kubernetes’ version, installed APIs, resources usages, etc.

#### Lease updates
Karmada will create a `Lease` object and a lease controller for each cluster when clusters are joined.

Each lease controller is responsible for updating the related Leases. The lease renewing time can be configured via `--cluster-lease-duration` and `--cluster-lease-renew-interval-fraction` flags(default is 10 seconds).

Lease’s updating process is independent with cluster’s status updating process, since cluster’s `.status` field is maintained by cluster status controller.

The cluster controller in Karmada control plane would check the state of each cluster every `--cluster-monitor-period` period(default is 5 seconds).

The cluster's `Ready` condition would be changed to `Unknown` when cluster controller has not heard from the cluster in the last `--cluster-monitor-grace-period`(default is 40 seconds).

### Check cluster status
You can use `kubectl` to check a Cluster's status and other details:
```
kubectl describe cluster <cluster-name>
```

The `Ready` condition in `Status` field indicates the cluster is healthy and ready to accept workloads.
It will be set to `False` if the cluster is not healthy and is not accepting workloads, and `Unknown` if the cluster controller has not heard from the cluster in the last `cluster-monitor-grace-period`.

The following example describes an unhealthy cluster:
```
kubectl describe cluster member1
 
Name:         member1
Namespace:    
Labels:       <none>
Annotations:  <none>
API Version:  cluster.karmada.io/v1alpha1
Kind:         Cluster
Metadata:
  Creation Timestamp:  2021-12-29T08:49:35Z
  Finalizers:
    karmada.io/cluster-controller
  Resource Version:  152047
  UID:               53c133ab-264e-4e8e-ab63-a21611f7fae8
Spec:
  API Endpoint:  https://172.23.0.7:6443
  Impersonator Secret Ref:
    Name:       member1-impersonator
    Namespace:  karmada-cluster
  Secret Ref:
    Name:       member1
    Namespace:  karmada-cluster
  Sync Mode:    Push
Status:
  Conditions:
    Last Transition Time:  2021-12-31T03:36:08Z
    Message:               cluster is not reachable
    Reason:                ClusterNotReachable
    Status:                False
    Type:                  Ready
Events:                    <none>
```

## Failover feature of Karmada
The failover feature is controlled by the `Failover` feature gate, users need to enable the `Failover` feature gate of karmada scheduler:
```
--feature-gates=Failover=true
```

### Concept

When it is determined that member clusters becoming unhealthy, the karmada scheduler will reschedule the reference application.
There are several constraints:
- For each rescheduled application, it still needs to meet the restrictions of PropagationPolicy, such as ClusterAffinity or SpreadConstraints.
- The application distributed on the ready clusters after the initial scheduling will remain when failover schedule.

#### Duplicated schedule type
For `Duplicated` schedule policy, when the number of candidate clusters that meet the PropagationPolicy restriction is not less than the number of failed clusters,
it will be rescheduled to candidate clusters according to the number of failed clusters. Otherwise, no rescheduling.

Take `Deployment` as example:
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        name: nginx
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
        - member3
        - member5
    spreadConstraints:
      - maxGroups: 2
        minGroups: 2
    replicaScheduling:
      replicaSchedulingType: Duplicated
```

Suppose there are 5 member clusters, and the initial scheduling result is in member1 and member2. When member2 fails, it triggers rescheduling.

It should be noted that rescheduling will not delete the application on the ready cluster member1. In the remaining 3 clusters, only member3 and member5 match the `clusterAffinity` policy.

Due to the limitations of spreadConstraints, the final result can be [member1, member3] or [member1, member5].

#### Divided schedule type
For `Divided` schedule policy, karmada scheduler will try to migrate replicas to the other health clusters.

Take `Deployment` as example:
```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        name: nginx
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        staticWeightList:
          - targetCluster:
              clusterNames:
                - member1
            weight: 1
          - targetCluster:
              clusterNames:
                - member2
            weight: 2
```

Karmada scheduler will divide the replicas according the `weightPreference`. The initial schedule result is member1 with 1 replica and member2 with 2 replicas.

When member1 fails, it triggers rescheduling. Karmada scheduler will try to migrate replicas to the other health clusters. The final result will be member2 with 3 replicas.
