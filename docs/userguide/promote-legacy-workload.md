# Promote legacy workload

Assume that there is a member cluster where a workload (like Deployment) is deployed but not managed by Karmada, we can use the `karmadactl promote` command to let Karmada take over this workload directly and not to cause its pods to restart.

## Example

### For member cluster in `Push` mode
There is an `nginx` Deployment that belongs to namespace `default` in member cluster `cluster1`.

```
[root@master1]# kubectl get cluster
NAME       VERSION   MODE   READY   AGE
cluster1   v1.22.3   Push   True    24d
```

```
[root@cluster1]# kubectl get deploy nginx
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   1/1     1            1           66s

[root@cluster1]# kubectl get pod
NAME                       READY   STATUS      RESTARTS   AGE
nginx-6799fc88d8-sqjj4     1/1     Running     0          2m12s
```

We can promote it to Karmada by executing the command below on the Karmada control plane.

```
[root@master1]# karmadactl promote deployment nginx -n default -c member1
Resource "apps/v1, Resource=deployments"(default/nginx) is promoted successfully
```

The nginx deployment has been adopted by Karmada.

```
[root@master1]# kubectl get deploy
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   1/1     1            1           7m25s
```

And the pod created by the nginx deployment in the member cluster wasn't restarted.

```
[root@cluster1]# kubectl get pod
NAME                       READY   STATUS      RESTARTS   AGE
nginx-6799fc88d8-sqjj4     1/1     Running     0          15m
```

### For member cluster in `Pull` mode
Most steps are same as those for clusters in `Push` mode. Only the flags of the `karmadactl promote` command are different.

```
karmadactl promote deployment nginx -n default -c cluster1 --cluster-kubeconfig=<CLUSTER_KUBECONFIG_PATH>
```

For more flags and example about the command, you can use `karmadactl promote --help`.

> Note: As the version upgrade of resources in Kubernetes is in progress, the apiserver of Karmada control plane cloud be different from member clusters. To avoid compatibility issues, you can specify the GVK of a resource, such as replacing `deployment` with `deployment.v1.apps`.