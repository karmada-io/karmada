# Nginx Deployment Example

This example demonstrates how to deploy an nginx application across multiple 
clusters using Karmada.

## Prerequisites

Before running this example, ensure you have:
- A Karmada control plane running (see [Quick Start](../../README.md#quick-start))
- At least one member cluster joined to Karmada
- `kubectl` configured to access the Karmada API server

```bash
# Verify you're connected to Karmada
kubectl config use-context karmada-apiserver

# Check member clusters are ready
kubectl get clusters
```

## What This Example Includes

1. **deployment.yaml** - A simple nginx deployment with 2 replicas
2. **propagationpolicy.yaml** - Policy to distribute the deployment to member clusters
3. **overridepolicy.yaml** - Policy to customize the deployment per cluster

## Step-by-Step Guide

### 1. Deploy the nginx application
```bash
kubectl create -f deployment.yaml
```

This creates a deployment in the Karmada control plane, but it won't be 
propagated to member clusters yet.

### 2. Create the propagation policy
```bash
kubectl create -f propagationpolicy.yaml
```

This tells Karmada to propagate the nginx deployment to your member clusters.

### 3. Verify the deployment
```bash
# Check in Karmada control plane
kubectl get deployment nginx

# Expected output:
# NAME    READY   UP-TO-DATE   AVAILABLE   AGE
# nginx   2/2     2            2           10s
```

### 4. (Optional) Apply override policy
```bash
kubectl create -f overridepolicy.yaml
```

This example shows how to customize deployments per cluster, such as adding 
labels or annotations.

## Verify in Member Clusters

To see the actual pods running in member clusters:
```bash
# Switch to member cluster context
export KUBECONFIG=$HOME/.kube/members.config
kubectl config use-context member1

# Check the pods
kubectl get pods -n default

# You should see nginx pods running
```

## Clean Up

To remove the example:
```bash
# Switch back to Karmada context
export KUBECONFIG=$HOME/.kube/karmada.config
kubectl config use-context karmada-apiserver

# Delete resources
kubectl delete -f propagationpolicy.yaml
kubectl delete -f deployment.yaml
```

## Learn More

- [Propagation Policy Documentation](https://karmada.io/docs/userguide/scheduling/resource-propagating)
- [Override Policy Documentation](https://karmada.io/docs/userguide/scheduling/override-policy)
