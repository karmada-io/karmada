# Descheduler

Users could divide their replicas of a workload into different clusters in terms of available resources of member clusters.
However, the scheduler's decisions are influenced by its view of Karmada at that point of time when a new `ResourceBinding` 
appears for scheduling. As Karmada multi-clusters are very dynamic and their state changes over time, there may be desire 
to move already running replicas to some other clusters due to lack of resources for the cluster. This may happen when 
some nodes of a cluster failed and the cluster does not have enough resource to accommodate their pods or the estimators 
have some estimation deviation, which is inevitable.

The karmada-descheduler will detect all deployments once in a while, every 2 minutes by default. In every period, it will find out 
how many unschedulable replicas a deployment has in target scheduled clusters by calling karmada-scheduler-estimator. Then 
it will evict them from decreasing `spec.clusters` and trigger karmada-scheduler to do a 'Scale Schedule' based on the current 
situation. Note that it will take effect only when the replica scheduling strategy is dynamic division.

## Prerequisites

### Karmada has been installed

We can install Karmada by referring to [quick-start](https://github.com/karmada-io/karmada#quick-start), or directly run `hack/local-up-karmada.sh` script which is also used to run our E2E cases.

### Member cluster component is ready

Ensure that all member clusters have joined Karmada and their corresponding karmada-scheduler-estimator is installed into karmada-host.

Check member clusters using the following command:

```bash
# check whether member clusters have joined
$ kubectl get cluster
NAME       VERSION   MODE   READY   AGE
member1    v1.19.1   Push   True    11m
member2    v1.19.1   Push   True    11m
member3    v1.19.1   Pull   True    5m12s

# check whether the karmada-scheduler-estimator of a member cluster has been working well
$ kubectl --context karmada-host -n karmada-system get pod | grep estimator
karmada-scheduler-estimator-member1-696b54fd56-xt789   1/1     Running   0          77s
karmada-scheduler-estimator-member2-774fb84c5d-md4wt   1/1     Running   0          75s
karmada-scheduler-estimator-member3-5c7d87f4b4-76gv9   1/1     Running   0          72s
```

- If a cluster has not joined, use `hack/deploy-agent-and-estimator.sh` to deploy both karmada-agent and karmada-scheduler-estimator.
- If the clusters have joined, use `hack/deploy-scheduler-estimator.sh` to only deploy karmada-scheduler-estimator.

### Scheduler option '--enable-scheduler-estimator'

After all member clusters have joined and estimators are all ready, specify the option `--enable-scheduler-estimator=true` to enable scheduler estimator.

```bash
# edit the deployment of karmada-scheduler
$ kubectl --context karmada-host -n karmada-system edit deployments.apps karmada-scheduler
```

Add the option `--enable-scheduler-estimator=true` into the command of container `karmada-scheduler`.

### Descheduler has been installed

Ensure that the karmada-descheduler has been installed onto karmada-host.

```bash
$ kubectl --context karmada-host -n karmada-system get pod | grep karmada-descheduler
karmada-descheduler-658648d5b-c22qf                    1/1     Running   0          80s
```

## Example

Let's simulate a replica scheduling failure in a member cluster due to lack of resources.

First we create a deployment with 3 replicas and divide them into 3 member clusters.

```yaml
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
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        dynamicWeight: AvailableReplicas
---
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
        resources:
          requests:
            cpu: "2"
```

It is possible for these 3 replicas to be evenly divided into 3 member clusters, that is, one replica in each cluster.
Now we taint all nodes in member1 and evict the replica.

```bash
# mark node "member1-control-plane" as unschedulable in cluster member1
$ kubectl --context member1 cordon member1-control-plane
# delete the pod in cluster member1
$ kubectl --context member1 delete pod -l app=nginx
```

A new pod will be created and cannot be scheduled by `kube-scheduler` due to lack of resources.

```bash
# the state of pod in cluster member1 is pending
$ kubectl --context member1 get pod
NAME                     READY   STATUS    RESTARTS   AGE
nginx-68b895fcbd-fccg4   1/1     Pending   0          80s
```

After about 5 to 7 minutes, the pod in member1 will be evicted and scheduled to other available clusters.

```bash
# get the pod in cluster member1
$ kubectl --context member1 get pod
No resources found in default namespace.
# get a list of pods in cluster member2
$ kubectl --context member2 get pod
NAME                     READY   STATUS    RESTARTS   AGE
nginx-68b895fcbd-dgd4x   1/1     Running   0          6m3s
nginx-68b895fcbd-nwgjn   1/1     Running   0          4s
```

