# Propagate dependencies
Deployment, Job, Pod, DaemonSet and StatefulSet dependencies (ConfigMaps, Secrets and ServiceAccounts) can be propagated to member
clusters automatically. This document demonstrates how to use this feature. For more design details, please refer to
[dependencies-automatically-propagation](../proposals/dependencies-automatically-propagation/README.md)

##Prerequisites
### Karmada has been installed

We can install Karmada by referring to [quick-start](https://github.com/karmada-io/karmada#quick-start), or directly run
`hack/local-up-karmada.sh` script which is also used to run our E2E cases.

### Enable PropagateDeps feature
```bash
kubectl edit deployment karmada-controller-manager -n karmada-system
```
Add `--feature-gates=PropagateDeps=true` option.

## Example
Create a Deployment mounted with a ConfigMap
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nginx
  labels:
    app: my-nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-nginx
  template:
    metadata:
      labels:
        app: my-nginx
    spec:
      containers:
      - image: nginx
        name: my-nginx
        ports:
        - containerPort: 80
        volumeMounts:
          - name: configmap
            mountPath: "/configmap"
      volumes:
        - name: configmap
          configMap:
            name: my-nginx-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-nginx-config
data:
  nginx.properties: |
    proxy-connect-timeout: "10s"
    proxy-read-timeout: "10s"
    client-max-body-size: "2m"
```
Create a propagation policy with this Deployment and set `propagateDeps: true`.
```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: my-nginx-propagation
spec:
  propagateDeps: true
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-nginx
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
            weight: 1
```
Upon successful policy execution, the Deployment and ConfigMap are properly propagated to the member cluster.
```bash
$  kubectl --kubeconfig /etc/karmada/karmada-apiserver.config get propagationpolicy
NAME                   AGE
my-nginx-propagation   16s
$  kubectl --kubeconfig /etc/karmada/karmada-apiserver.config get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
my-nginx   2/2     2            2           22m
# member cluster1
$ kubectl config use-context member1
Switched to context "member1".
$  kubectl get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
my-nginx   1/1     1            1           25m
$  kubectl get configmap
NAME               DATA   AGE
my-nginx-config    1      26m
# member cluster2
$ kubectl config use-context member2
Switched to context "member2".
$ kubectl get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
my-nginx   1/1     1            1           27m
$  kubectl get configmap
NAME               DATA   AGE
my-nginx-config    1      27m
```
