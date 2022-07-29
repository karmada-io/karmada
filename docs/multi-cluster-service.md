# Multi-cluster Service Discovery

Users are able to **export** and **import** services between clusters with [Multi-cluster Service APIs](https://github.com/kubernetes-sigs/mcs-api).

## Prerequisites

### Karmada has been installed

We can install Karmada by referring to [Quick Start](https://github.com/karmada-io/karmada#quick-start), or directly run `hack/local-up-karmada.sh` script which is also used to run our E2E cases.

### Member Cluster Network

Ensure that at least two clusters have been added to Karmada, and the container networks between member clusters are connected.

- If you use the `hack/local-up-karmada.sh` script to deploy Karmada, Karmada will have three member clusters, and the container networks of the `member1` and `member2` will be connected.
- You can use `Submariner` or other related open source projects to connected networks between member clusters.

### The ServiceExport and ServiceImport CRDs have been installed

We need to install ServiceExport and ServiceImport in the member clusters.

After ServiceExport and ServiceImport have been installed on the **Karmada Control Plane**, we can create `ClusterPropagationPolicy` to propagate those two CRDs to the member clusters.

```yaml
# propagate ServiceExport CRD
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: serviceexport-policy
spec:
  resourceSelectors:
    - apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      name: serviceexports.multicluster.x-k8s.io
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
---        
# propagate ServiceImport CRD
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: serviceimport-policy
spec:
  resourceSelectors:
    - apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      name: serviceimports.multicluster.x-k8s.io
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```
## Example

### Step 1: Deploy service on the `member1` cluster 

We need to deploy service on the `member1` cluster for discovery.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: serve
spec:
  replicas: 1
  selector:
    matchLabels:
      app: serve
  template:
    metadata:
      labels:
        app: serve
    spec:
      containers:
      - name: serve
        image: jeremyot/serve:0a40de8
        args:
        - "--message='hello from cluster member1 (Node: {{env \"NODE_NAME\"}} Pod: {{env \"POD_NAME\"}} Address: {{addr}})'"
        env:
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
---      
apiVersion: v1
kind: Service
metadata:
  name: serve
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: serve
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: mcs-workload
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: serve
    - apiVersion: v1
      kind: Service
      name: serve
  placement:
    clusterAffinity:
      clusterNames:
        - member1
```

### Step 2: Export service to the `member2` cluster

- Create a `ServiceExport` object on **Karmada Control Plane**, and then create a `PropagationPolicy` to propagate the `ServiceExport` object to the `member1` cluster.

  ```yaml
  apiVersion: multicluster.x-k8s.io/v1alpha1
  kind: ServiceExport
  metadata:
    name: serve
  ---
  apiVersion: policy.karmada.io/v1alpha1
  kind: PropagationPolicy
  metadata:
    name: serve-export-policy
  spec:
    resourceSelectors:
      - apiVersion: multicluster.x-k8s.io/v1alpha1
        kind: ServiceExport
        name: serve
    placement:
      clusterAffinity:
        clusterNames:
          - member1
  ```

- Create a `ServiceImport` object on **Karmada Control Plane**, and then create a `PropagationPolicy` to propagate the `ServiceImport` object to the `member2` cluster.

  ```yaml
  apiVersion: multicluster.x-k8s.io/v1alpha1
  kind: ServiceImport
  metadata:
    name: serve
  spec:
    type: ClusterSetIP
    ports:
    - port: 80
      protocol: TCP
  ---
  apiVersion: policy.karmada.io/v1alpha1
  kind: PropagationPolicy
  metadata:
    name: serve-import-policy
  spec:
    resourceSelectors:
      - apiVersion: multicluster.x-k8s.io/v1alpha1
        kind: ServiceImport
        name: serve
    placement:
      clusterAffinity:
        clusterNames:
          - member2
  ```

### Step 3: Access the service from `member2` cluster

After the above steps, we can find the **derived service** which has the prefix `derived-` on the `member2` cluster. Then, we can access the **derived service** to access the service on the `member1` cluster.
```shell
# get the services in cluster member2, and we can find the service with the name 'derived-serve'
$ kubectl --kubeconfig ~/.kube/members.config --context member2 get svc
NAME            TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
derived-serve   ClusterIP   10.13.205.2   <none>        80/TCP    81s
kubernetes      ClusterIP   10.13.0.1     <none>        443/TCP   15m
```

Start a pod `request` on the `member2` cluster to access the ClusterIP of **derived service**:

```shell
$ kubectl --kubeconfig ~/.kube/members.config --context member2 run -i --rm --restart=Never --image=jeremyot/request:0a40de8 request -- --duration={duration-time} --address={ClusterIP of derived service}
```

Eg, if we continue to access service for 3s, ClusterIP is `10.13.205.2`:

```shell
# access the service of derived-serve, and the pod in member1 cluster returns a response
$ kubectl --kubeconfig ~/.kube/members.config --context member2 run -i --rm --restart=Never --image=jeremyot/request:0a40de8 request -- --duration=3s --address=10.13.205.2
If you don't see a command prompt, try pressing enter.
2022/07/24 15:13:08 'hello from cluster member1 (Node: member1-control-plane Pod: serve-9b5b94f65-cp87p Address: 10.10.0.5)'
2022/07/24 15:13:09 'hello from cluster member1 (Node: member1-control-plane Pod: serve-9b5b94f65-cp87p Address: 10.10.0.5)'
pod "request" deleted
```

