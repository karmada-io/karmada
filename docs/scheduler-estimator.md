# Cluster Accurate Scheduler Estimator

Users could divide their replicas of a workload into different clusters in terms of available resources of member clusters. When some clusters are lack of resources, scheduler would not assign excessive replicas into these clusters by calling karmada-scheduler-estimator.

## Prerequisites

### Karmada has been installed

We can install Karmada by referring to [quick-start](https://github.com/karmada-io/karmada#quick-start), or directly run `hack/local-up-karmada.sh` script which is also used to run our E2E cases.

### Member cluster component is ready

Ensure that all member clusters have been joined and their corresponding karmada-scheduler-estimator is installed into karmada-host.

You could check by using the following command:

```bash
# check whether the member cluster has been joined
$ kubectl get cluster
NAME       VERSION   MODE   READY   AGE
member1    v1.19.1   Push   True    11m
member2    v1.19.1   Push   True    11m
member3    v1.19.1   Pull   True    5m12s

# check whether the karmada-scheduler-estimator of a member cluster has been working well
$ kubectl --context karmada-host get pod -n karmada-system | grep estimator
karmada-scheduler-estimator-member1-696b54fd56-xt789   1/1     Running   0          77s
karmada-scheduler-estimator-member2-774fb84c5d-md4wt   1/1     Running   0          75s
karmada-scheduler-estimator-member3-5c7d87f4b4-76gv9   1/1     Running   0          72s
```

- If the cluster has not been joined, you could use `hack/deploy-agent-and-estimator.sh` to deploy both karmada-agent and karmada-scheduler-estimator.
- If the cluster has been joined already, you could use `hack/deploy-scheduler-estimator.sh` to only deploy karmada-scheduler-estimator.

### Scheduler option '--enable-scheduler-estimator'

After all member clusters have been joined and estimators are all ready, please specify the option `--enable-scheduler-estimator=true` to enable scheduler estimator.

```bash
# edit the deployment of karmada-scheduler
$ kubectl --context karmada-host edit -n karmada-system deployments.apps karmada-scheduler
```

And then add the option `--enable-scheduler-estimator=true` into the command of container `karmada-scheduler`.

## Example

Now we could divide the replicas into different member clusters. Note that `propagationPolicy.spec.replicaScheduling.replicaSchedulingType` must be `Divided` and `propagationPolicy.spec.replicaScheduling.replicaDivisionPreference` must be `Aggregated`. The scheduler will try to divide the replicas aggregately in terms of all available resources of member clusters.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: aggregated-policy
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
      replicaSchedulingType: Divided
      replicaDivisionPreference: Aggregated
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 5
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
        ports:
        - containerPort: 80
          name: web-1
        resources:
          requests:
            cpu: "1"
            memory: 2Gi
```

You will find all replicas have been assigned to as few clusters as possible.

```
$ kubectl get deployments.apps          
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   5/5     5            5           2m16s
$ kubectl get rb nginx-deployment -o=custom-columns=NAME:.metadata.name,CLUSTER:.spec.clusters  
NAME               CLUSTER
nginx-deployment   [map[name:member1 replicas:5] map[name:member2] map[name:member3]]
```

After that, we change the resource request of the deployment to a large number and have a try again.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 5
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
        ports:
        - containerPort: 80
          name: web-1
        resources:
          requests:
            cpu: "100"
            memory: 200Gi
```

As any node of member clusters does not have so many cpu and memory resources, we will find workload scheduling failed.

```bash
$ kubectl get deployments.apps 
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   0/5     0            0           2m20s
$ kubectl get rb nginx-deployment -o=custom-columns=NAME:.metadata.name,CLUSTER:.spec.clusters  
NAME               CLUSTER
nginx-deployment   <none>
```
