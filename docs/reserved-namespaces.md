# Reserved Namespaces

> Note: Avoid creating namespaces with the prefix `kube-` and `karmada-`, since they are reserved for Kubernetes
> and Karmada system namespaces.
> For now, resources under the following namespaces will not be propagated:

- namespaces prefix `kube-`(including but not limited to `kube-system`, `kube-public`, `kube-node-lease`)
- karmada-system
- karmada-cluster
- karmada-es-*