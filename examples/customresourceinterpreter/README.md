# Examples

## Resource Interpreter

This example installs a CustomResourceDefinition(CRD), `Workload` in the `karmada-apiserver`, and deploys a deployment `karmada-interpreter-webhook-example` in the Karmada host.

### Install

### Prerequisites

For Karmada deploy using `hack/local-up-karmada.sh`, there are `karmada-host`, `karmada-apiserver` and three member clusters named `member1`, `member2` and `member3`.

> Note: If you use other installation methods, please adapt your installation method proactively.

In the current example, we will deploy `karmada-interpreter-webhook-example` in the Karmada control plane. Since it involves a cluster with Pull mode, the initiator of resource interpreter requests is in `karmada-agent`. Therefore, we create a Service of type `LoadBalancer` for `karmada-interpreter-webhook-example`.

So we need to deploy MetalLB as a Load Balancer to expose the webhook. Please run the following script to deploy MetalLB.

```bash
kubectl --context="karmada-host" get configmap kube-proxy -n kube-system -o yaml | \
  sed -e "s/strictARP: false/strictARP: true/" | \
  kubectl --context="karmada-host" apply -n kube-system -f -

curl https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml -k | \
  sed '0,/args:/s//args:\n        - --webhook-mode=disabled/' | \
  sed '/apiVersion: admissionregistration/,$d' | \
  kubectl --context="karmada-host" apply -f -

export interpreter_webhook_example_service_external_ip_address=$(kubectl config view --template='{{range $_, $value := .clusters }}{{if eq $value.name "karmada-apiserver"}}{{$value.cluster.server}}{{end}}{{end}}' | \
  awk -F/ '{print $3}' | \
  sed 's/:.*//' | \
  awk -F. '{printf "%s.%s.%s.8",$1,$2,$3}')

cat <<EOF | kubectl --context="karmada-host" apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: metallb-config
  namespace: metallb-system
spec:
  addresses:
  - ${interpreter_webhook_example_service_external_ip_address}-${interpreter_webhook_example_service_external_ip_address}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: metallb-advertisement
  namespace: metallb-system
EOF
```

#### Step1: Install `Workload` CRD in `karmada-apiserver` and member clusters

Install CRD in `karmada-apiserver` by running the following command:

```bash
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml
```

Create a `ClusterPropagationPolicy` resource object to propagate `Workload` CRD to member clusters:

<details>

<summary>workload-crd-cpp.yaml</summary>

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy
metadata:
  name: workload-crd-cpp
spec:
  resourceSelectors:
    - apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      name: workloads.workload.example.io
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
        - member3
```
</details>

```bash
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f workload-crd-cpp.yaml
```

#### Step2: Deploy webhook configuration in karmada-apiserver

Execute below script:

<details>

<summary>webhook-configuration.sh</summary>

```bash
#!/usr/bin/env bash

export ca_string=$(cat ${HOME}/.karmada/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
export temp_path=$(mktemp -d)
export interpreter_webhook_example_service_external_ip_address=$(kubectl config view --template='{{range $_, $value := .clusters }}{{if eq $value.name "karmada-apiserver"}}{{$value.cluster.server}}{{end}}{{end}}' | \
  awk -F/ '{print $3}' | \
  sed 's/:.*//' | \
  awk -F. '{printf "%s.%s.%s.8",$1,$2,$3}')

cp -rf "examples/customresourceinterpreter/webhook-configuration.yaml" "${temp_path}/temp.yaml"
sed -i'' -e "s/{{caBundle}}/${ca_string}/g" -e "s/{{karmada-interpreter-webhook-example-svc-address}}/${interpreter_webhook_example_service_external_ip_address}/g" "${temp_path}/temp.yaml"
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f "${temp_path}/temp.yaml"
rm -rf "${temp_path}"
```

</details>

```bash
chmod +x webhook-configuration.sh

./webhook-configuration.sh
```

#### Step3: Deploy karmada-interpreter-webhook-example in karmada-host

```bash
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-host apply -f examples/customresourceinterpreter/karmada-interpreter-webhook-example.yaml
```

> Note: karmada-interpreter-webhook-example is just a demo for testing and reference. If you plan to use the interpreter webhook, please implement specific components based on your business needs.

In the current example, the interpreter webhook is deployed under the namespace `karmada-system`. If you are trying to deploy the interpreter webhook in a namespace other than the default karmada-system namespace, and use the domain address of Service in the URL. Such as (take the `test` namespace as an example):

```yaml
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterWebhookConfiguration
metadata:
  name: examples
webhooks:
  - name: workloads.example.com
    rules:
      - operations: [ "InterpretReplica","ReviseReplica","Retain","AggregateStatus", "InterpretHealth", "InterpretStatus", "InterpretDependency" ]
        apiGroups: [ "workload.example.io" ]
        apiVersions: [ "v1alpha1" ]
        kinds: [ "Workload" ]
    clientConfig:
      url: https://karmada-interpreter-webhook-example.test.svc.cluster.local:443/interpreter-workload # domain address here
      caBundle: {{caBundle}}
    interpreterContextVersions: [ "v1alpha1" ]
    timeoutSeconds: 3
```

Please set the correct certificate and add the domain address to the CN field of the certificate.

In the testing environment of Karmada, this is controlled in script `hack/deploy-karmada.sh`:

https://github.com/karmada-io/karmada/blob/303f2cd24bf5d750c2391bb6699ac89d78b3c43f/hack/deploy-karmada.sh#L155

We recommend that you deploy the interpreter webhook component and Karmada control plane components in the same namespace. If you need to deploy them in different namespaces, please plan ahead when generating certificates.

The relevant problem description has been recorded in [#4478](https://github.com/karmada-io/karmada/issues/4478), please refer to it.

### Usage

Create a `Workload` resource and propagate it to the member clusters:

<details>

<summary>workload-interpret-test.yaml</summary>

```yaml
apiVersion: workload.example.io/v1alpha1
kind: Workload
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 3
  paused: false
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        name: nginx
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-workload-propagation
spec:
  resourceSelectors:
    - apiVersion: workload.example.io/v1alpha1
      kind: Workload
      name: nginx
  placement:
    clusterAffinity:
      clusterNames:
        - member1
        - member2
        - member3
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Divided
      weightPreference:
        staticWeightList:
          - targetCluster:
              clusterNames:
                - member1
            weight: 1
          - targetCluster:
              clusterNames:
                - member2
            weight: 1
          - targetCluster:
              clusterNames:
                - member3
            weight: 1
```

</details>

```bash
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f workload-interpret-test.yaml
```

#### InterpretReplica

You can get `ResourceBinding` to check if the `replicas` field is interpreted successfully.

```bash
kubectl get rb nginx-workload -o yaml
```

#### ReviseReplica

You can check if the replicas field of `Workload` object is revised to 1 in all member clusters.

```bash
kubectl --kubeconfig $HOME/.kube/members.config --context member1 get workload nginx --template={{.spec.replicas}}
```

#### Retain

Update `spec.paused` of `Workload` object in member1 cluster to `true`.

```bash
kubectl --kubeconfig $HOME/.kube/members.config --context member1 patch workload nginx --type='json' -p='[{"op": "replace", "path": "/spec/paused", "value":true}]'
```

Check if it is retained successfully.
```bash
kubectl --kubeconfig $HOME/.kube/members.config --context member1 get workload nginx --template={{.spec.paused}}
```

#### InterpretStatus

There is no `Workload` controller deployed on member clusters, so in order to simulate the `Workload` CR handling, 
we will manually update `status.readyReplicas` of `Workload` object in member1 cluster to 1. 

```bash
kubectl proxy --port=8001 &
curl  http://127.0.0.1:8001/apis/workload.example.io/v1alpha1/namespaces/default/workloads/nginx/status  -XPATCH -d'{"status":{"readyReplicas": 1}}' -H "Content-Type: application/merge-patch+json
```

Then you can get `ResourceBinding` to check if the `status.aggregatedStatus[x].status` field is interpreted successfully.

```bash
kubectl get rb nginx-workload --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver -o yaml
```

You can also check the `status.manifestStatuses[x].status` field of Karmada `Work` object in namespace karmada-es-member1.

#### InterpretHealth

You can get `ResourceBinding` to check if the `status.aggregatedStatus[x].health` field is interpreted successfully.

```bash
kubectl get rb nginx-workload --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver -o yaml
```

You can also check the `status.manifestStatuses[x].health` field of Karmada `Work` object in namespace karmada-es-member1.

#### AggregateStatus

You can check if the `status` field of `Workload` object is aggregated correctly.

```bash
kubectl get workload nginx --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver -o yaml
```

> Note: If you want to use `Retain`/`InterpretStatus`/`InterpretHealth` function in Pull mode cluster, you need to deploy karmada-interpreter-webhook-example in the Pull mode cluster.