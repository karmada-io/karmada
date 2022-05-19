# Multi-cluster Ingress

Users can use [MultiClusterIngress API](https://github.com/karmada-io/karmada/blob/master/pkg/apis/networking/v1alpha1/ingress_types.go) provided in Karmada to import external traffic to services in the member clusters.

## Prerequisites

### Karmada has been installed

We can install Karmada by referring to [quick-start](https://github.com/karmada-io/karmada#quick-start), or directly run `hack/local-up-karmada.sh` script which is also used to run our E2E cases.

### Cluster Network

Currently, we need to use the [MCS](https://github.com/karmada-io/karmada/blob/master/docs/multi-cluster-service.md#the-serviceexport-and-serviceimport-crds-have-been-installed) feature to import external traffic.

So we need to ensure that the container networks between the **host cluster** and member clusters are connected. The **host cluster** indicates the cluster where the Karmada control plane is deployed.

- If you use the `hack/local-up-karmada.sh` script to deploy Karmada, Karmada will have three member clusters, and the container networks between the **host cluster**, `member1` and `member2` are connected.
- You can use `Submariner` or other related open source projects to connected networks between clusters.

## Example

### Step 1: Deploy ingress-nginx on the host cluster

We use [multi-cluster-ingress-nginx](https://github.com/karmada-io/multi-cluster-ingress-nginx) as the demo for demonstration. We've made some changes based on the latest version(controller-v1.1.1) of [ingress-nginx](https://github.com/kubernetes/ingress-nginx).

#### Download code

```
// for HTTPS
git clone https://github.com/karmada-io/multi-cluster-ingress-nginx.git
// for SSH
git clone git@github.com:karmada-io/multi-cluster-ingress-nginx.git
```

#### Build and deploy ingress-nginx

Using the existing `karmada-host` kind cluster to build and deploy the ingress controller.

```
export KIND_CLUSTER_NAME=karmada-host
kubectl config use-context karmada-host
make dev-env
```

#### Apply kubeconfig secret

Create a secret that contains the `karmada-apiserver` authentication credential in the following format:

```yaml
# karmada-kubeconfig-secret.yaml
apiVersion: v1
data:
  kubeconfig: {data} # Base64-encoded
kind: Secret
metadata:
  name: kubeconfig
  namespace: ingress-nginx
type: Opaque
```

You can get the authentication credential from the `/root/.kube/karmada.config` file, or use command:

```
kubectl -n karmada-system get secret kubeconfig -oyaml | grep kubeconfig | sed -n '1p' | awk '{print $2}'
```

Then apply yaml:

```
kubectl apply -f karmada-kubeconfig-secret.yaml
```

#### Edit ingress-nginx-controller deployment

We want `nginx-ingress-controller` to access `karmada-apiserver` to listen to changes in resources(such as multiclusteringress, endpointslices, and service). Therefore, we need to mount the authentication credential of `karmada-apiserver` to the `nginx-ingress-controller`.

```
kubectl -n ingress-nginx edit deployment ingress-nginx-controller
```

Edit as follows:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  ...
spec:
  ...
  template:
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
        - --karmada-kubeconfig=/etc/kubeconfig  # new line
        ...
        volumeMounts:
        ...
        - mountPath: /etc/kubeconfig            # new line
          name: kubeconfig                      # new line
          subPath: kubeconfig                   # new line
      volumes:
	  ...
      - name: kubeconfig                        # new line
        secret:                                 # new line
          secretName: kubeconfig                # new line
```

### Step 2: Use the MCS feature to discovery service

#### Install ServiceExport and ServiceImport CRDs

Refer to [here](https://github.com/karmada-io/karmada/blob/master/docs/multi-cluster-service.md#the-serviceexport-and-serviceimport-crds-have-been-installed).

#### Deploy web on member1 cluster

deploy.yaml:

<details>

<summary>unfold me to see the yaml</summary>

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: hello-app
        image: gcr.io/google-samples/hello-app:1.0
        ports:
        - containerPort: 8080
          protocol: TCP
---      
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  ports:
  - port: 81
    targetPort: 8080
  selector:
    app: web
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: mcs-workload
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: web
    - apiVersion: v1
      kind: Service
      name: web
  placement:
    clusterAffinity:
      clusterNames:
        - member1
```

</details>

```
kubectl --context karmada-apiserver apply -f deploy.yaml
```

#### Export web service from member1 cluster

service_export.yaml:

<details>

<summary>unfold me to see the yaml</summary>

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ServiceExport
metadata:
  name: web
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: web-export-policy
spec:
  resourceSelectors:
    - apiVersion: multicluster.x-k8s.io/v1alpha1
      kind: ServiceExport
      name: web
  placement:
    clusterAffinity:
      clusterNames:
        - member1
```

</details>

```
kubectl --context karmada-apiserver apply -f service_export.yaml
```

#### Import web service to member2 cluster

service_import.yaml:

<details>

<summary>unfold me to see the yaml</summary>

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ServiceImport
metadata:
  name: web
spec:
  type: ClusterSetIP
  ports:
  - port: 81
    protocol: TCP
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: web-import-policy
spec:
  resourceSelectors:
    - apiVersion: multicluster.x-k8s.io/v1alpha1
      kind: ServiceImport
      name: web
  placement:
    clusterAffinity:
      clusterNames:
        - member2
```

</details>

```
kubectl --context karmada-apiserver apply -f service_import.yaml
```

### Step 3: Deploy multiclusteringress on karmada-controlplane

mci-web.yaml:

<details>

<summary>unfold me to see the yaml</summary>

```yaml
apiVersion: networking.karmada.io/v1alpha1
kind: MultiClusterIngress
metadata:
  name: demo-localhost
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: demo.localdev.me
    http:
      paths:
      - backend:
          service:
            name: web
            port:
              number: 81
        path: /web
        pathType: Prefix
```

</details>

```
kubectl --context karmada-apiserver apply -f 
```

### Step 4: Local testing

Let's forward a local port to the ingress controller:

```
kubectl port-forward --namespace=ingress-nginx service/ingress-nginx-controller 8080:80
```

At this point, if you access http://demo.localdev.me:8080/web/, you should see an HTML page telling you:

```html
Hello, world!
Version: 1.0.0
Hostname: web-xxx-xxx
```