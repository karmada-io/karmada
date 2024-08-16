# Karmada hack GuideLines

This document describes how you can use the scripts from the [`hack`](.) directory
and gives a brief introduction and explanation of these scripts.

Scripts in this directory are mainly for the purpose which improves development efficiency and
ensures development quality.

# Key scripts

## For end-user

- [`local-up-karmada.sh`](local-up-karmada.sh) This script will quickly set up a local development environment with member clusters based on the current codebase.

- [`local-down-karmada.sh`](local-down-karmada.sh) This script will clean up the whole local deployment environment installed by the previous `local-up-karmada.sh` script.

- [`remote-up-karmada.sh`](remote-up-karmada.sh) This script will install Karmada to a standalone K8s cluster, this cluster
  may be real, remote , and even for production. It is worth noting for the connectivity from your client to Karmada API server,
  it will directly use host network by default, else `export LOAD_BALANCER=true` with the `LoadBalancer` type service before the following script.
  If your want to customize a load balancer service, you may add the annotations at the metadata part of service `karmada-apiserver` in
  [`../artifacts/deploy/karmada-apiserver.yaml`](../artifacts/deploy/karmada-apiserver.yaml) before the installing. The
  following is an example.
```
  # If you want to use a internal IP in public cloud you need to fill the following annotation, 
  # For the more annotation settings please read your public cloud docs
  annotations: 
    # Aliyun cloud
    #service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: "intranet"
    # Huawei cloud
    #kubernetes.io/elb.autocreate: '{"type":"inner"}'
    # Tencent cloud (you need to replace words 'xxxxxxxx')
    #service.kubernetes.io/qcloud-loadbalancer-internal-subnetid: subnet-xxxxxxxx
```
  The usage of `remote-up-karmada.sh`:
```
# hack/remote-up-karmada.sh <kubeconfig> <context_name>
```
`kubeconfig` is your cluster's kubeconfig that you want to install to

`context_name` is the name of context in 'kubeconfig'

- [`deploy-karmada-agent.sh`](deploy-karmada-agent.sh) This script will install Karmada Agent to the specific cluster.

- [`deploy-scheduler-estimator.sh`](deploy-scheduler-estimator.sh) This script will only install Karmada Scheduler 
  Estimator to the specific cluster. Please follow the [instruction](../docs/scheduler-estimator.md) for more details.

- [`deploy-agent-and-estimator.sh`](deploy-agent-and-estimator.sh) This script will install Karmada Agent and Karmada 
  Scheduler Estimator to the specific cluster together. If applied, there is no need to use the extra `deploy-karmada-agent.sh`
  and `deploy-scheduler-estimator.sh` script.

- [`undeploy-karmada.sh`](undeploy-karmada.sh) This script will uninstall Karmada from the specific cluster.
  It will uninstall Karmada from your local environment default. If you installed Karmada with `remote-up-karmada.sh`,
  please use it like this: `hack/undeploy-karmada.sh <KUBECONFIG> <CONTEXT_NAME>`, the same parameters as you input at
  the installing step.

- [`delete-cluster.sh`](delete-cluster.sh) This script delete a kube cluster by kind,
  please use it like this: `hack/delete-cluster.sh.sh <CLUSTER_NAME> <KUBECONFIG>`

## For CI pipeline
- [`local-up-karmada.sh`](local-up-karmada.sh) This script also used for testing.

- [`run-e2e.sh`](run-e2e.sh) This script runs e2e test against on Karmada control plane. You should prepare your environment
  in advance with `local-up-karmada.sh`.

## Some internal scripts
These scripts are not intended used by end-users, just for the development
- [`deploy-karmada.sh`](deploy-karmada.sh) Underlying common implementation for `local-up-karmada.sh` and `remote-up-karmada.sh`.

- [`util.sh`](util.sh) All util functions.
