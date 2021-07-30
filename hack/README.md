# Karmada hack GuideLines

This document describes how you can use the scripts from the [`hack`](.) directory
and gives a brief introduction and explanation of these scripts.

Scripts in this directory are mainly for the purpose which improves development efficiency and
ensures development quality.

# Key scripts

## For end-user

- [`local-up-karmada.sh`](local-up-karmada.sh) This script will quickly set up a local development environment based on the current codebase.
  
- [`remote-up-karmada.sh`](remote-up-karmada.sh) This script will install Karmada to a standalone K8s cluster, this cluster
  may be real, remote , and even for production. It is worth noting for the connectivity from your client to Karmada API server,
  it will create a load balancer service with an external IP by default, if your want to customize this service, you may add 
  the annotations at the metadata part of service `karmada-apiserver` in 
  [`../artifacts/deploy/karmada-apiserver.yaml`](../artifacts/deploy/karmada-apiserver.yaml) before the installing. The 
  following is an example.
```
  # If you want to use a internal IP in public cloud you need to fill the following annotation, 
  # Fot the more annotation settings please read your public cloud docs
  annotations: 
    # Aliyun cloud
    #service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: "intranet"
    # Huawei cloud
    #kubernetes.io/elb.autocreate: '{"type":"inner"}'
    # Tencent cloud (you need to replace words 'xxxxxxxx')
    #service.kubernetes.io/qcloud-loadbalancer-internal-subnetid: subnet-xxxxxxxx
```

- [`deploy-karmada-agent.sh`](deploy-karmada-agent.sh) This script will install Karmada Agent to the specific cluster.

- [`undeploy-karmada.sh`](undeploy-karmada.sh) This script will uninstall Karmada from the specific cluster. 
  It will uninstall Karmada from your local environment default. If you installed Karmada with `remote-up-karmada.sh`, 
  please use it like this: `hack/undeploy-karmada.sh <KUBECONFIG> <CONTEXT_NAME>`, the same parameters as you input at 
  the installing step.
  
## For CI pipeline
- [`karmada-bootstrap.sh`](karmada-bootstrap.sh) This script will quickly pull up a local Karmada environment too, 
  what is different from `local-up-karmada.sh` is it will pull up member clusters. This is usually for testing,
  of course, you may also use it for your local environment.

- [`run-e2e.sh`](run-e2e.sh) This script runs e2e test against on Karmada control plane. You should prepare your environment
  in advance with `karmada-bootstrap.sh`.
  
## Some internal scripts
These scripts are not intended used by end-users, just for the development
- [`deploy-karmada.sh`](deploy-karmada.sh) Underlying common implementation for `local-up-karmada.sh`, `remote-up-karmada.sh` 
  and `karmada-bootstrap.sh`
  
- [`util.sh`](util.sh) All util functions.
