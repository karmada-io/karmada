# Examples

## Resource Interpreter

This example implements a new CustomResourceDefinition(CRD), `Workload`, and creates a resource interpreter webhook.

### Install

> For karmada deployed using `hack/local-up-karmada.sh`, there are `karmada-host`, `karmada-apiserver` and three member clusters named `member1`, `member2` and `member3`.

#### Step1: Install `Workload` CRD in `karmada-apiserver` and member clusters

Install CRD in `karmada-apiserver` by running the following command:

```bash
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml
```

Create `ClusterPropagationPolicy` object to propagate CRD to member clusters:

workload-crd-cpp.yaml:

<details>

<summary>unfold me to see the yaml</summary>

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

webhook-configuration.sh

<details>

<summary>unfold me to see the script</summary>

```bash
#!/usr/bin/env bash

export ca_string=$(cat ${HOME}/.karmada/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
export temp_path=$(mktemp -d)

cp -rf "examples/customresourceinterpreter/webhook-configuration.yaml" "${temp_path}/temp.yaml"
sed -i'' -e "s/{{caBundle}}/${ca_string}/g" "${temp_path}/temp.yaml"
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-apiserver apply -f "${temp_path}/temp.yaml"
rm -rf "${temp_path}"
```

</details>

```bash
chmod +x webhook-configuration.sh

./webhook-configuration.sh
```

#### Step3: Deploy interpreter webhook example in karmada-host

```bash
kubectl --kubeconfig $HOME/.kube/karmada.config --context karmada-host apply -f examples/customresourceinterpreter/karmada-interpreter-webhook-example.yaml
```

### Usage

Create a `Workload` resource and propagate it to the member clusters:

workload-interpret-test.yaml:

<details>

<summary>unfold me to see the yaml</summary>

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

> Note: If you want to use `Retain` function in pull mode cluster, you need to deploy interpreter webhook example in this member cluster.