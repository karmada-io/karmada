<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Installing Karmada](#installing-karmada)
  - [Prerequisites](#prerequisites)
    - [Karmada kubectl plugin](#karmada-kubectl-plugin)
  - [Install Karmada by Karmada command-line tool](#install-karmada-by-karmada-command-line-tool)
    - [Install Karmada on your own cluster](#install-karmada-on-your-own-cluster)
      - [Offline installation](#offline-installation)
      - [Deploy HA](#deploy-ha)
    - [Install Karmada in Kind cluster](#install-karmada-in-kind-cluster)
  - [Install Karmada by Helm Chart Deployment](#install-karmada-by-helm-chart-deployment)
  - [Install Karmada from source](#install-karmada-from-source)
  - [Install Karmada for development environment](#install-karmada-for-development-environment)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Installing Karmada

## Prerequisites

### Karmada kubectl plugin
`kubectl-karmada` is the Karmada command-line tool that lets you control the Karmada control plane, it presents as
the [kubectl plugin][1].
For installation instructions see [installing kubectl-karmada](./install-kubectl-karmada.md).

## Install Karmada by Karmada command-line tool

### Install Karmada on your own cluster

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
------------------------------------------------------------------------------------------------------
 █████   ████   █████████   ███████████   ██████   ██████   █████████   ██████████     █████████
░░███   ███░   ███░░░░░███ ░░███░░░░░███ ░░██████ ██████   ███░░░░░███ ░░███░░░░███   ███░░░░░███
 ░███  ███    ░███    ░███  ░███    ░███  ░███░█████░███  ░███    ░███  ░███   ░░███ ░███    ░███
 ░███████     ░███████████  ░██████████   ░███░░███ ░███  ░███████████  ░███    ░███ ░███████████
 ░███░░███    ░███░░░░░███  ░███░░░░░███  ░███ ░░░  ░███  ░███░░░░░███  ░███    ░███ ░███░░░░░███
 ░███ ░░███   ░███    ░███  ░███    ░███  ░███      ░███  ░███    ░███  ░███    ███  ░███    ░███
 █████ ░░████ █████   █████ █████   █████ █████     █████ █████   █████ ██████████   █████   █████
░░░░░   ░░░░ ░░░░░   ░░░░░ ░░░░░   ░░░░░ ░░░░░     ░░░░░ ░░░░░   ░░░░░ ░░░░░░░░░░   ░░░░░   ░░░░░
------------------------------------------------------------------------------------------------------
Karmada is installed successfully.

Register Kubernetes cluster to Karmada control plane.

Register cluster with 'Push' mode
                                                                                                                                                                             
Step 1: Use karmadactl join to register the cluster to Karmada control panel. --cluster-kubeconfig is members kubeconfig.
(In karmada)~# MEMBER_CLUSTER_NAME=`cat ~/.kube/config  | grep current-context | sed 's/: /\n/g'| sed '1d'`
(In karmada)~# karmadactl --kubeconfig /etc/karmada/karmada-apiserver.config  join ${MEMBER_CLUSTER_NAME} --cluster-kubeconfig=$HOME/.kube/config

Step 2: Show members of karmada
(In karmada)~# kubectl  --kubeconfig /etc/karmada/karmada-apiserver.config get clusters


Register cluster with 'Pull' mode

Step 1:  Send karmada kubeconfig and karmada-agent.yaml to member kubernetes
(In karmada)~# scp /etc/karmada/karmada-apiserver.config /etc/karmada/karmada-agent.yaml {member kubernetes}:~
                                                                                                                                                                             
Step 2:  Create karmada kubeconfig secret
 Notice:
   Cross-network, need to change the config server address.
(In member kubernetes)~#  kubectl create ns karmada-system
(In member kubernetes)~#  kubectl create secret generic karmada-kubeconfig --from-file=karmada-kubeconfig=/root/karmada-apiserver.config  -n karmada-system                  

Step 3: Create karmada agent
(In member kubernetes)~#  MEMBER_CLUSTER_NAME="demo"
(In member kubernetes)~#  sed -i "s/{member_cluster_name}/${MEMBER_CLUSTER_NAME}/g" karmada-agent.yaml
(In member kubernetes)~#  kubectl apply -f karmada-agent.yaml
                                                                                                                                                                             
Step 4: Show members of karmada                                                                                                                                              
(In karmada)~# kubectl  --kubeconfig /etc/karmada/karmada-apiserver.config get clusters

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

Use `--crds` flag to specify the CRD file. e.g.
```bash
kubectl karmada init --crds /$HOME/crds.tar.gz
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

### Install Karmada in Kind cluster

> kind is a tool for running local Kubernetes clusters using Docker container "nodes". 
> It was primarily designed for testing Kubernetes itself, not for production.

Create a cluster named `host` by `hack/create-cluster.sh`:
```bash
hack/create-cluster.sh host $HOME/.kube/host.config
```

Install Karmada v1.2.0 by command `kubectl karmada init`:
```bash
kubectl karmada init --crds https://github.com/karmada-io/karmada/releases/download/v1.2.0/crds.tar.gz --kubeconfig=$HOME/.kube/host.config
```

Check installed components:
```bash
kubectl get pods -n karmada-system --kubeconfig=$HOME/.kube/host.config
NAME                                           READY   STATUS    RESTARTS   AGE
etcd-0                                         1/1     Running   0          2m55s
karmada-aggregated-apiserver-84b45bf9b-n5gnk   1/1     Running   0          109s
karmada-apiserver-6dc4cf6964-cz4jh             1/1     Running   0          2m40s
karmada-controller-manager-556cf896bc-79sxz    1/1     Running   0          2m3s
karmada-scheduler-7b9d8b5764-6n48j             1/1     Running   0          2m6s
karmada-webhook-7cf7986866-m75jw               1/1     Running   0          2m
kube-controller-manager-85c789dcfc-k89f8       1/1     Running   0          2m10s
```

## Install Karmada by Helm Chart Deployment
Please refer to [installing by Helm](https://github.com/karmada-io/karmada/tree/master/charts/karmada).

## Install Karmada by binary
Please refer to [installing by binary](https://github.com/karmada-io/karmada/blob/master/docs/installation/binary-install.md).

## Install Karmada from source

Please refer to [installing from source](./fromsource.md).

[1]: https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/

## Install Karmada for development environment

If you want to try Karmada, we recommend that build a development environment by 
`hack/local-up-karmada.sh` which will do following tasks for you:
- Start a Kubernetes cluster by [kind](https://kind.sigs.k8s.io/) to run the Karmada control plane, aka. the `host cluster`.
- Build Karmada control plane components based on a current codebase.
- Deploy Karmada control plane components on the `host cluster`.
- Create member clusters and join Karmada.

**1. Clone Karmada repo to your machine:**
```
git clone https://github.com/karmada-io/karmada
```
or use your fork repo by replacing your `GitHub ID`:
```
git clone https://github.com/<GitHub ID>/karmada
```

**2. Change to the karmada directory:**
```
cd karmada
```

**3. Deploy and run Karmada control plane:**

run the following script:

```
hack/local-up-karmada.sh
```
This script will do following tasks for you:
- Start a Kubernetes cluster to run the Karmada control plane, aka. the `host cluster`.
- Build Karmada control plane components based on a current codebase.
- Deploy Karmada control plane components on the `host cluster`.
- Create member clusters and join Karmada.

If everything goes well, at the end of the script output, you will see similar messages as follows:
```
Local Karmada is running.

To start using your Karmada environment, run:
  export KUBECONFIG="$HOME/.kube/karmada.config"
Please use 'kubectl config use-context karmada-host/karmada-apiserver' to switch the host and control plane cluster.

To manage your member clusters, run:
  export KUBECONFIG="$HOME/.kube/members.config"
Please use 'kubectl config use-context member1/member2/member3' to switch to the different member cluster.
```

**4. Check registered cluster**

```
kubectl get clusters --kubeconfig=/$HOME/.kube/karmada.config
```

You will get similar output as follows:
```
NAME      VERSION   MODE   READY   AGE
member1   v1.23.4   Push   True    7m38s
member2   v1.23.4   Push   True    7m35s
member3   v1.23.4   Pull   True    7m27s
```

There are 3 clusters named `member1`, `member2` and `member3` have registered with `Push` or `Pull` mode.
