# Use Flux to support Helm chart propagation

[Flux](https://fluxcd.io/) is most useful when used as a deployment tool at the end of a Continuous Delivery pipeline. Flux will make sure that your new container images and config changes are propagated to the cluster.

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
    [root@10-6-201-150 ~]# kubectl get pod -n flux-system
    NAME                                       READY   STATUS    RESTARTS   AGE
    helm-controller-55896d6ccf-dlf8b           1/1     Running   0          15d
    kustomize-controller-76795877c9-mbrsk      1/1     Running   0          15d
    notification-controller-7ccfbfbb98-lpgjl   1/1     Running   0          15d
    source-controller-6b8d9cb5cc-7dbcb         1/1     Running   0          15d
    ```

## Helm chart propagation

1. Define a HelmRepository source

   ```yaml
   apiVersion: source.toolkit.fluxcd.io/v1beta1
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
       - apiVersion: source.toolkit.fluxcd.io/v1beta1
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
   [root@10-6-201-150 helm]# kubectl apply -f ../helm/
   helmrelease.helm.toolkit.fluxcd.io/podinfo created
   helmrepository.source.toolkit.fluxcd.io/podinfo created
   propagationpolicy.policy.karmada.io/helm-release created
   propagationpolicy.policy.karmada.io/helm-repo created
   ```

5. Switch to the distributed cluster

   ```
   [root@10-6-201-150 ~]# kubectl config use-context member2
   Switched to context "member2".
   [root@10-6-201-150 ~]# kubectl get pod
   NAME                      READY   STATUS    RESTARTS   AGE
   podinfo-78c475b77-94x54   1/1     Running   0          104s
   [root@10-6-201-150 ~]# helm list -A
   NAME   	NAMESPACE	REVISION	UPDATED                               	STATUS  	CHART        	APP VERSION
   podinfo	default  	1       	2022-05-21 07:14:50.73460681 +0000 UTC	deployed	podinfo-5.0.3	5.0.3
   [root@10-6-201-150 ~]#
   ```

## Kustomize propagation

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
   [root@10-6-201-150 flux]# kubectl apply -f kust/
   gitrepository.source.toolkit.fluxcd.io/podinfo created
   kustomization.kustomize.toolkit.fluxcd.io/podinfo-dev created
   propagationpolicy.policy.karmada.io/kust-git created
   propagationpolicy.policy.karmada.io/kust-release created
   ```

5. Switch to the distributed cluster

   ```
   [root@10-6-201-150 ~]# kubectl get pod -n dev
   NAME                        READY   STATUS    RESTARTS   AGE
   backend-69c7655cb-rbtrq     1/1     Running   0          15s
   cache-bdff5c8dc-mmnbm       1/1     Running   0          15s
   frontend-7f98bf6f85-dw4vq   1/1     Running   0          15s
   ```

## Reference
- https://fluxcd.io
- https://github.com/fluxcd
