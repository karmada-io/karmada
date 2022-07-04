# kubectl-karmada Installation

You can install `kubectl-karmada` plug-in in any of the following ways:

- Download from the release.
- Install using Krew.
- Build from source code.

## Prerequisites

### kubectl
`kubectl` is the Kubernetes command line tool lets you control Kubernetes clusters.
For installation instructions see [installing kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

## Download from the release

Karmada provides `kubectl-karmada` plug-in download service since v0.9.0. You can choose proper plug-in version which fits your operator system form [karmada release](https://github.com/karmada-io/karmada/releases).

Take v0.9.0 that working with linux-amd64 os as an example:

```bash
wget https://github.com/karmada-io/karmada/releases/download/v0.9.0/kubectl-karmada-linux-amd64.tar.gz

tar -zxf kubectl-karmada-linux-amd64.tar.gz
```

Next, move `kubectl-karmada` executable file to `PATH` path, reference from [Installing kubectl plugins](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/#installing-kubectl-plugins).

## Install using Krew

Krew is the plugin manager for `kubectl` command-line tool.

[Install and set up](https://krew.sigs.k8s.io/docs/user-guide/setup/install/) Krew on your machine.

Then install `kubectl-karmada` plug-in:

```bash
kubectl krew install karmada
```

You can refer to [Quickstart of Krew](https://krew.sigs.k8s.io/docs/user-guide/quickstart/) for more information.

## Build from source code

Clone karmada repo and run `make` cmd from the repository:

```bash
make kubectl-karmada
```

Next, move the `kubectl-karmada` executable file under the `_output` folder in the project root directory to the `PATH` path.
