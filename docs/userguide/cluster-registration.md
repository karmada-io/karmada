<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Cluster Registration](#cluster-registration)
  - [Overview of cluster mode](#overview-of-cluster-mode)
    - [Push mode](#push-mode)
    - [Pull mode](#pull-mode)
  - [Register cluster with 'Push' mode](#register-cluster-with-push-mode)
    - [Register cluster by CLI](#register-cluster-by-cli)
    - [Check cluster status](#check-cluster-status)
    - [Unregister cluster by CLI](#unregister-cluster-by-cli)
  - [Register cluster with 'Pull' mode](#register-cluster-with-pull-mode)
    - [Register cluster](#register-cluster)
    - [Check cluster status](#check-cluster-status-1)
    - [Unregister cluster](#unregister-cluster)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Cluster Registration

## Overview of cluster mode

Karmada supports both `Push` and `Pull` modes to manage the member clusters.
The main difference between `Push` and `Pull` modes is the way access to member clusters when deploying manifests.

### Push mode
Karmada control plane will access member cluster's `kube-apiserver` directly to get cluster status and deploy manifests.

### Pull mode
Karmada control plane will not access member cluster but delegate it to an extra component named `karmada-agent`.

Each `karmada-agent` serves for a cluster and take responsibility for:
- Register cluster to Karmada(creates the `Cluster` object)
- Maintains cluster status and reports to Karmada(updates the status of `Cluster` object)
- Watch manifests from Karmada execution space(namespace, `karmada-es-<cluster name>`) and deploy to the cluster which serves for.

## Register cluster with 'Push' mode

You can use the [kubectl-karmada](./../installation/install-kubectl-karmada.md) CLI to `join`(register) and `unjoin`(unregister) clusters.

### Register cluster by CLI

Join cluster with name `member1` to Karmada by using the following command.
```
kubectl karmada join member1 --kubeconfig=<karmada kubeconfig> --cluster-kubeconfig=<member1 kubeconfig>
```
Repeat this step to join any additional clusters.

The `--kubeconfig` specifies the Karmada's `kubeconfig` file and the CLI infers `karmada-apiserver` context
from the `current-context` field of the `kubeconfig`. If there are more than one context is configured in
the `kubeconfig` file, it is recommended to specify the context by the `--karmada-context` flag. For example:
```
kubectl karmada join member1 --kubeconfig=<karmada kubeconfig> --karmada-context=karmada --cluster-kubeconfig=<member1 kubeconfig>
```

The `--cluster-kubeconfig` specifies the member cluster's `kubeconfig` and the CLI infers the member cluster's context
by the cluster name. If there is more than one context is configured in the `kubeconfig` file, or you don't want to use
the context name to register, it is recommended to specify the context by the `--cluster-context` flag. For example:

```
kubectl karmada join member1 --kubeconfig=<karmada kubeconfig> --karmada-context=karmada \
--cluster-kubeconfig=<member1 kubeconfig> --cluster-context=member1
```
> Note: The registering cluster name can be different from the context with `--cluster-context` specified.

### Check cluster status

Check the status of the joined clusters by using the following command.
```
kubectl get clusters

NAME      VERSION   MODE   READY   AGE
member1   v1.20.7   Push   True    66s
```
### Unregister cluster by CLI

You can unjoin clusters by using the following command.
```
kubectl karmada unjoin member1 --kubeconfig=<karmada kubeconfig> --cluster-kubeconfig=<member1 kubeconfig>
```
During unjoin process, the resources propagated to `member1` by Karmada will be cleaned up.
And the `--cluster-kubeconfig` is used to clean up the secret created at the `join` phase.

Repeat this step to unjoin any additional clusters.

## Register cluster with 'Pull' mode

### Register cluster

After `karmada-agent` be deployed, it will register cluster automatically at the start-up phase.

### Check cluster status

Check the status of the registered clusters by using the same command above.
```
kubectl get clusters
NAME      VERSION   MODE   READY   AGE
member3   v1.20.7   Pull   True    66s
```
### Unregister cluster

Undeploy the `karmada-agent` and then remove the `cluster` manually from Karmada.
```
kubectl delete cluster member3
```
