# Use Flux to support Helm chart propagation

[Flux](https://fluxcd.io/) is most useful when used as a deployment tool at the end of a Continuous Delivery pipeline. Flux will make sure that your new container images and config changes are propagated to the cluster. With Flux, Karmada can easily realize the ability to distribute applications packaged by helm across clusters. Not only that, with Karmada's OverridePolicy, users can customize applications for specific clusters and manage cross-cluster applications on the unified Karmada control plane.

## Start up Karmada clusters
You just need to clone the Karmada repo, and run the following script in the `karmada` directory. 

```
hack/local-up-karmada.sh
```

## Start up Flux 

Install the `flux` binary:

```
curl -s https://fluxcd.io/install.sh | sudo bash 
```

Install the toolkit controllers in the `flux-system` namespace:

```
flux install
```

Tips:

 1. The Flux toolkit controllers need to be installed on each cluster using the `flux install` command.

 2. If the Flux toolkit controllers are successfully installed, you should see the following Pods:

```
$ kubectl get pod -n flux-system
NAME                                       READY   STATUS    RESTARTS   AGE
helm-controller-55896d6ccf-dlf8b           1/1     Running   0          15d
kustomize-controller-76795877c9-mbrsk      1/1     Running   0          15d
notification-controller-7ccfbfbb98-lpgjl   1/1     Running   0          15d
source-controller-6b8d9cb5cc-7dbcb         1/1     Running   0          15d
```

## Helm chart propagation

If you want to propagate helm applications to member clusters, you can refer to the guide below.

1. Define a HelmRepository source

```yaml
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: HelmRepository
metadata:
  name: podinfo
spec:
  interval: 1m
  url: https://stefanprodan.github.io/podinfo     
--- 
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: helm-repo
spec:
  resourceSelectors:
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: HelmRepository
      name: podinfo
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

2. Define a HelmRelease source

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: podinfo
spec:
  interval: 5m
  chart:
    spec:
      chart: podinfo
      version: 5.0.3
      sourceRef:
        kind: HelmRepository
        name: podinfo
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: helm-release
spec:
  resourceSelectors:
    - apiVersion: helm.toolkit.fluxcd.io/v2beta1
      kind: HelmRelease
      name: podinfo
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

3. Apply those YAMLs to the karmada-apiserver

```
$ kubectl apply -f ../helm/
helmrelease.helm.toolkit.fluxcd.io/podinfo created
helmrepository.source.toolkit.fluxcd.io/podinfo created
propagationpolicy.policy.karmada.io/helm-release created
propagationpolicy.policy.karmada.io/helm-repo created
```

4. Switch to the distributed cluster

```
helm --kubeconfig=/root/.kube/members.config --kube-context member1 list
NAME   	NAMESPACE	REVISION	UPDATED                               	STATUS  	CHART        	APP VERSION
podinfo	default  	1       	2022-05-27 01:44:35.24229175 +0000 UTC	deployed	podinfo-5.0.3	5.0.3
```

Also, you can use Karmada's OverridePolicy to customize applications for specific clusters. For example, if you just want to change replicas in member1, you can refer to the overridePolicy below.

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: OverridePolicy
metadata:
name: example-override
namespace: default
spec:
resourceSelectors:
  - apiVersion: helm.toolkit.fluxcd.io/v2beta1
    kind: HelmRelease
    name: podinfo
overrideRules:
  - targetCluster:
      clusterNames:
        - member1
    overriders:
      plaintext:
        - path: "/spec/values"
          operator: add
          value:
            replicaCount: 2
```

After that, you can find that replicas in member1 has changed.

```
$ kubectl --kubeconfig ~/.kube/members.config --context member1 get po
NAME                       READY   STATUS    RESTARTS   AGE
podinfo-68979685bc-6wz6s   1/1     Running   0          6m28s
podinfo-68979685bc-dz9f6   1/1     Running   0          7m42s
```

## Kustomize propagation

Kustomize propagation is basically the same as helm chart propagation above. You can refer to the guide below.

1. Define a Git repository source

```yaml
apiVersion: source.toolkit.fluxcd.io/v1beta1
kind: GitRepository
metadata:
name: podinfo
spec:
  interval: 1m
  url: https://github.com/stefanprodan/podinfo
  ref:
    branch: master
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
name: kust-git
spec:
resourceSelectors:
  - apiVersion: source.toolkit.fluxcd.io/v1beta1
    kind: GitRepository
    name: podinfo
placement:
  clusterAffinity:
    clusterNames:
      - member1
      - member2
```

2. Define a kustomization

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: podinfo-dev
spec:
  interval: 5m
  path: "./deploy/overlays/dev/"
  prune: true
  sourceRef:
    kind: GitRepository
    name: podinfo
  validation: client
  timeout: 80s
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: kust-release
spec:
  resourceSelectors:
    - apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
      kind: Kustomization
      name: podinfo-dev
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
```

3. Apply those YAMLs to the karmada-apiserver

```
$ kubectl apply -f kust/
gitrepository.source.toolkit.fluxcd.io/podinfo created
kustomization.kustomize.toolkit.fluxcd.io/podinfo-dev created
propagationpolicy.policy.karmada.io/kust-git created
propagationpolicy.policy.karmada.io/kust-release created
```

4. Switch to the distributed cluster

```
$ kubectl get pod -n dev
NAME                        READY   STATUS    RESTARTS   AGE
backend-69c7655cb-rbtrq     1/1     Running   0          15s
cache-bdff5c8dc-mmnbm       1/1     Running   0          15s
frontend-7f98bf6f85-dw4vq   1/1     Running   0          15s
```

## Reference
- https://fluxcd.io
- https://github.com/fluxcd
