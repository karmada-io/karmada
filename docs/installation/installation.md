<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Installing Karmada](#installing-karmada)
  - [Prerequisites](#prerequisites)
    - [Karmada kubectl plugin](#karmada-kubectl-plugin)
  - [Install Karmada by Karmada command-line tool](#install-karmada-by-karmada-command-line-tool)
    - [Install Karmada in Kubernetes](#install-karmada-in-kubernetes)
      - [Offline installation](#offline-installation)
      - [Deploy HA](#deploy-ha)
  - [Install Karmada by Helm Chart Deployment](#install-karmada-by-helm-chart-deployment)
  - [Install Karmada from source](#install-karmada-from-source)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Installing Karmada

## Prerequisites

### Karmada kubectl plugin
`kubectl-karmada` is the Karmada command-line tool that lets you control the Karmada control plane, it presents as
the [kubectl plugin][1].
For installation instructions see [installing kubectl-karmada](./install-kubectl-karmada.md).

## Install Karmada by Karmada command-line tool

### Install Karmada in Kubernetes

Assume you have put your cluster's `kubeconfig` file to `$HOME/.kube/config` or specify the path
with `KUBECONFIG` environment variable. Otherwise, you should specify the configuration file by
setting `--kubeconfig` flag to the following commands.

> Note: The `init` command is available from v1.0. 

Run the following command to install:
```bash
kubectl karmada init
```
It might take about 5 minutes and if everything goes well, you will see outputs similar to:
```
I1216 07:37:45.862959    4256 cert.go:230] Generate ca certificate success.
I1216 07:37:46.000798    4256 cert.go:230] Generate etcd-server certificate success.
...
...
┌──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
| Push mode                                                                                                                                                                    |
|                                                                                                                                                                              |
| Step 1: Member kubernetes join karmada control plane                                                                                                                         |
|                                                                                                                                                                              |
| (In karmada)~#  cat ~/.kube/config  | grep current-context | sed 's/: /\n/g'| sed '1d' #MEMBER_CLUSTER_NAME                                                                  |
| (In karmada)~# kubectl-karmada  --kubeconfig /etc/karmada/karmada-apiserver.config  join ${MEMBER_CLUSTER_NAME} --cluster-kubeconfig=$HOME/.kube/config                      |
|                                                                                                                                                                              |
| Step 2: Create member kubernetes kubeconfig secret                                                                                                                           |
|                                                                                                                                                                              |
| (In member kubernetes)~# kubectl create ns karmada-system                                                                                                                    |
| (In member kubernetes)~# kubectl create secret generic ${MEMBER_CLUSTER_NAME}-kubeconfig --from-file=${MEMBER_CLUSTER_NAME}-kubeconfig=$HOME/.kube/config  -n karmada-system |
|                                                                                                                                                                              |
| Step 3: Create karmada scheduler estimator                                                                                                                                   |
|                                                                                                                                                                              |
| (In member kubernetes)~# sed -i "s/{{member_cluster_name}}/${MEMBER_CLUSTER_NAME}/g" /etc/karmada/karmada-scheduler-estimator.yaml                                           |
| (In member kubernetes)~# kubectl create -f  /etc/karmada/karmada-scheduler-estimator.yaml                                                                                    |
|                                                                                                                                                                              |
| Step 4: Show members of karmada                                                                                                                                              |
|                                                                                                                                                                              |
| (In karmada)~# kubectl  --kubeconfig /etc/karmada/karmada-apiserver.config get clusters                                                                                      |
|                                                                                                                                                                              |
├── —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— —— ┤
| Pull mode                                                                                                                                                                    |
|                                                                                                                                                                              |
| Step 1:  Send karmada kubeconfig and karmada-agent.yaml to member kubernetes                                                                                                 |
|                                                                                                                                                                              |
| (In karmada)~# scp /etc/karmada/karmada-apiserver.config /etc/karmada/karmada-agent.yaml {member kubernetes}:~                                                               |
|                                                                                                                                                                              |
| Step 2:  Create karmada kubeconfig secret                                                                                                                                    |
|  Notice:                                                                                                                                                                     |
|    Cross-network, need to change the config server address.                                                                                                                  |
|                                                                                                                                                                              |
| (In member kubernetes)~#  kubectl create ns karmada-system                                                                                                                   |
| (In member kubernetes)~#  kubectl create secret generic karmada-kubeconfig --from-file=karmada-kubeconfig=/root/karmada-apiserver.config  -n karmada-system                  |
|                                                                                                                                                                              |
| Step 3: Create karmada agent                                                                                                                                                 |
|                                                                                                                                                                              |
| (In member kubernetes)~#  MEMBER_CLUSTER_NAME="demo"                                                                                                                         |
| (In member kubernetes)~#  sed -i "s/{member_cluster_name}/${MEMBER_CLUSTER_NAME}/g" karmada-agent.yaml                                                                       |
| (In member kubernetes)~#  kubectl create -f karmada-agent.yaml                                                                                                               |
|                                                                                                                                                                              |
| Step 4: Show members of karmada                                                                                                                                              |
|                                                                                                                                                                              |
| (In karmada)~# kubectl  --kubeconfig /etc/karmada/karmada-apiserver.config get clusters                                                                                      |
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

The components of Karmada are installed in `karmada-system` namespace by default, you can get them by:
```bash
kubectl get deployments -n karmada-system
NAME                           READY   UP-TO-DATE   AVAILABLE   AGE
karmada-aggregated-apiserver   1/1     1            1           102s
karmada-apiserver              1/1     1            1           2m34s
karmada-controller-manager     1/1     1            1           116s
karmada-scheduler              1/1     1            1           119s
karmada-webhook                1/1     1            1           113s
kube-controller-manager        1/1     1            1           2m3s
```
And the `karmada-etcd` is installed as the `StatefulSet`, get it by:
```bash
kubectl get statefulsets -n karmada-system
NAME   READY   AGE
etcd   1/1     28m
```

The configuration file of Karmada will be created to `/etc/karmada/karmada-apiserver.config` by default.

#### Offline installation

When installing Karmada, the `kubectl karmada init` will download the APIs(CRD) from the Karmada official release page
(e.g. `https://github.com/karmada-io/karmada/releases/tag/v0.10.1`) and load images from the official registry by default.

If you want to install Karmada offline, maybe you have to specify the APIs tar file as well as the image.

Use `--crd` flag to specify the CRD file. e.g.
```bash
kubectl karmada init --crd /$HOME/crds.tar.gz
```

The images of Karmada components could be specified, take `karmada-controller-manager` as an example:
```bash
kubectl karmada init --karmada-controller-manager-image=example.registry.com/library/karmada-controller-manager:1.0 
```

#### Deploy HA
Use `--karmada-apiserver-replicas` and `--etcd-replicas` flags to specify the number of the replicas (defaults to `1`).
```bash
kubectl karmada init --karmada-apiserver-replicas 3 --etcd-replicas 3
```

## Install Karmada by Helm Chart Deployment
Please refer to [installing by Helm](https://github.com/karmada-io/karmada/tree/master/charts).

## Install Karmada from source

Please refer to [installing from source](./fromsource.md).

[1]: https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/