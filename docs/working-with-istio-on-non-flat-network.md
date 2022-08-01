# working-with-istio-on-non-flat-network

This document uses an example to demonstrate how to use [Istio](https://istio.io/) on Karmada when the clusters reside
on the different networks.

Follow this guide to install the Istio control plane on `member1` (the primary cluster) and configure `member2` (the
remote cluster) to use the control plane in `member1`. All clusters reside on the different network, meaning there is
not direct connectivity between the pods in all clusters.

<image src="images/istio-on-karmada-different-network.png" caption="Istio on Karmada-different-network" />

***
The reason for deploying `istiod` on the `member1` is that `kiali` needs to be deployed on the same cluster as `istiod`
. If `istiod` and `kiali` are deployed on the `karmada-host`,`kiali` will not find the namespace created by `karmada`. It
cannot implement the function of service topology for application deployed by `karmada`. I will continue to provide a new
solution later that deploys `istiod` on the `karmada-host`.
***

## Install Karmada

### Install karmada control plane

Following the steps [Install karmada control plane](https://github.com/karmada-io/karmada#install-karmada-control-plane)
in Quick Start, you can get a Karmada.

## Deploy Istio

***
If you are testing multicluster setup on `kind` you can use [MetalLB](https://metallb.universe.tf/installation/) to make use of `EXTERNAL-IP` for `LoadBalancer` services.
***

### Install istioctl

Please refer to the [istioctl](https://istio.io/latest/docs/setup/getting-started/#download) Installation.

### Prepare CA certificates

Following the
steps [plug-in-certificates-and-key-into-the-cluster](https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/#plug-in-certificates-and-key-into-the-cluster)
to configure Istio CA.

Replace the cluster name `cluster1` with `primary`, the output will looks like as following:

```bash
[root@vm1-su-001 istio-1.12.6]# tree certs/
certs/
├── primary
│   ├── ca-cert.pem
│   ├── ca-key.pem
│   ├── cert-chain.pem
│   └── root-cert.pem
├── root-ca.conf
├── root-cert.csr
├── root-cert.pem
├── root-cert.srl
└── root-key.pem
```

### Install Istio on karmada-apiserver

Export `KUBECONFIG` and switch to `karmada apiserver`:

```bash
export KUBECONFIG=$HOME/.kube/karmada.config
kubectl config use-context karmada-apiserver
```

Create a secret `cacerts` in `istio-system` namespace:

```bash
kubectl create namespace istio-system
kubectl create secret generic cacerts -n istio-system \
    --from-file=certs/primary/ca-cert.pem \
    --from-file=certs/primary/ca-key.pem \
    --from-file=certs/primary/root-cert.pem \
    --from-file=certs/primary/cert-chain.pem
```

Create a propagation policy for `cacerts` secret:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: cacerts-propagation
  namespace: istio-system
spec:
  resourceSelectors:
    - apiVersion: v1
      kind: Secret
      name: cacerts
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
EOF
```

Override namespace `istio-system` label on `member1`:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterOverridePolicy
metadata:
  name: istio-system-member1
spec:
  resourceSelectors:
    - apiVersion: v1
      kind: Namespace
      name: istio-system
  overrideRules:
    - targetCluster:
        clusterNames:
          - member1
      overriders:
        plaintext:
          - path: "/metadata/labels"
            operator: add
            value:
              topology.istio.io/network: network1
EOF
```

Override namespace `istio-system` label on `member2`:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterOverridePolicy
metadata:
  name: istio-system-member2
spec:
  resourceSelectors:
    - apiVersion: v1
      kind: Namespace
      name: istio-system
  overrideRules:
    - targetCluster:
        clusterNames:
          - member2
      overriders:
        plaintext:
          - path: "/metadata/labels"
            operator: add
            value:
              topology.istio.io/network: network2
EOF
```

Run the following command to install istio CRDs on karmada apiserver:

```bash
istioctl manifest generate --set profile=external \
  --set values.global.configCluster=true \
  --set values.global.externalIstiod=false \
  --set values.global.defaultPodDisruptionBudget.enabled=false \
  --set values.telemetry.enabled=false | kubectl apply -f -
```

### Install Istiod on member1

1. Install istio control plane

Export `KUBECONFIG` and switch to `member1`:

```bash
export KUBECONFIG="$HOME/.kube/members.config"
kubectl config use-context member1
```

```bash
cat <<EOF | istioctl install -y -f -
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  meshConfig:
    accessLogFile: /dev/stdout
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: member1
      network: network1
EOF
```

2. Install the east-west gateway in `member1`

```bash
samples/multicluster/gen-eastwest-gateway.sh --mesh mesh1 --cluster member1 --network network1 | istioctl install -y -f -
```

3. Expose the control plane and service in `member1`

```bash
kubectl apply -f samples/multicluster/expose-istiod.yaml -n istio-system
kubectl apply -f samples/multicluster/expose-services.yaml -n istio-system
```

### Configure `member2` as a remote cluster

1. Enable API ServerAccess to `member2`

switch to `member2`:

```bash
kubectl config use-context member2
```

Prepare member2 cluster secret

```bash
istioctl x create-remote-secret --name=member2 > istio-remote-secret-member2.yaml
```

Switch to `member1`:

```bash
kubectl config use-context member1
```

Apply istio remote secret

```bash
kubectl apply -f istio-remote-secret-member2.yaml
```

2. Configure member2 as a remote

Save the address of `member1`’s east-west gateway

```bash
export DISCOVERY_ADDRESS=$(kubectl -n istio-system get svc istio-eastwestgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

Create a remote configuration on `member2`.

Switch to `member2`:

```bash
kubectl config use-context member2
```

```bash
cat <<EOF | istioctl install -y -f -
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: member2
      network: network2
      remotePilotAddress: ${DISCOVERY_ADDRESS}
EOF
```

3. Install the east-west gateway in `member2`

```bash
samples/multicluster/gen-eastwest-gateway.sh --mesh mesh1 --cluster member2 --network network2 | istioctl install -y -f -
```

### Deploy bookinfo application

See module [Deploy bookinfo application](./working-with-istio-on-flat-network.md#deploy-bookinfo-application.md)