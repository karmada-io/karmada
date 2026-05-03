# Karmada Troubleshooting Guide

## Propagation Chain

The Karmada propagation chain flows as:

```
Resource Template (Deployment, Service, etc.)
    ↓  [matched by]
PropagationPolicy / ClusterPropagationPolicy
    ↓  [creates]
ResourceBinding / ClusterResourceBinding
    ↓  [scheduler assigns clusters]
ResourceBinding.spec.clusters = [member1, member2]
    ↓  [execution controller creates]
Work (one per target cluster, in karmada-es-<cluster> namespace)
    ↓  [cluster agent syncs]
Resource on member cluster
```

## Common Failure Points

### 1. Resource Not Matched by Policy

**Symptom**: Resource template exists in Karmada control plane but no ResourceBinding is created.

**Check**:
- Does a PropagationPolicy or ClusterPropagationPolicy exist with matching `resourceSelectors`?
- Are `apiVersion`, `kind` exactly correct? (case-sensitive)
- Is `name` field set but doesn't match the resource name?
- Is `namespace` field set correctly? (PropagationPolicy is namespace-scoped; ClusterPropagationPolicy can match all)
- Is `labelSelector` matching? (only evaluated if `name` is empty)

**Debug commands**:
```bash
kubectl get propagationpolicy -A
kubectl get clusterpropagationpolicy
kubectl describe propagationpolicy <name> -n <ns>
kubectl describe clusterpropagationpolicy <name>
```

### 2. ResourceBinding Created but Not Scheduled

**Symptom**: ResourceBinding exists but `spec.clusters` is empty or `status.conditions[type=Scheduled]=False`.

**Check**:
- ResourceBinding conditions: `kubectl describe resourcebinding <name> -n <ns>`
- Reason codes:
  - `NoClusterFit`: No cluster matches the placement constraints.
  - `Unschedulable`: Clusters match but lack resources.
  - `SchedulerError`: Internal scheduler error.
  - `QuotaExceeded`: FederatedResourceQuota limit exceeded.
- Are target clusters available? `kubectl get clusters`
- Do clusters have taints that conflict with `clusterTolerations`?
- Are `spreadConstraints` satisfiable? (e.g., need 2 regions but only 1 available)
- Is scheduling suspended? Check `spec.suspension.scheduling`.

**Debug commands**:
```bash
kubectl get resourcebinding -A
kubectl get resourcebinding <name> -n <ns> -o yaml
```

### 3. Work Not Created for a Target Cluster

**Symptom**: ResourceBinding has clusters in `spec.clusters` but no Work object exists for cluster X.

**Check**:
- Is dispatching suspended? Check `spec.suspension.dispatching` on the ResourceBinding.
- Is the specific cluster suspended? Check `spec.suspension.dispatchingOnClusters`.
- Is the execution controller running? `kubectl get pods -n karmada-system | grep karmada-controller-manager`
- Check controller logs: `kubectl logs -n karmada-system deployment/karmada-controller-manager`

**Debug commands**:
```bash
# Find works for a binding
kubectl get work -l "resourcebinding.karmada.io/name=<binding-name>" -A
# Check work in cluster execution namespace
kubectl get work -n karmada-es-<cluster>
```

### 4. Work Created but Resource Not Applied on Member Cluster

**Symptom**: Work exists but `status.conditions[type=Applied]=False`.

**Check**:
- Work status conditions:
  - `Applied=False`: The resource was not applied. Check `AppliedMessage` for error.
  - `Dispatching=False`: Dispatching suspended at Work level (`spec.suspendDispatching=true`).
- Conflict resolution: `spec.conflictResolution=Abort` (default) and resource already exists on the member cluster.
- RBAC: Does the cluster agent have permission to create the resource?
- Check cluster agent connectivity: `kubectl get pods -n karmada-system -l app=karmada-agent`

**Debug commands**:
```bash
kubectl describe work <name> -n karmada-es-<cluster>
kubectl get work <name> -n karmada-es-<cluster> -o yaml
```

### 5. Conflict Resolution Abort

**Symptom**: Resource exists on member cluster, propagation aborted.

**Solution**: Set `conflictResolution: Overwrite` on the PropagationPolicy, OR manually delete the conflicting resource on the member cluster.

### 6. OverridePolicy Not Applied

**Symptom**: Resource propagated but overrides not taking effect.

**Check**:
- Does the OverridePolicy `resourceSelectors` match the resource?
- Does the override's `targetCluster` match the cluster?
- Override order matters: ImageOverrider → Command → Args → Labels → Annotations → Field → Plaintext.
- Multiple OverridePolicies may conflict (last writer wins for same field).

**Debug commands**:
```bash
kubectl get overridepolicy -A
kubectl get clusteroverridepolicy
kubectl describe overridepolicy <name> -n <ns>
```

### 7. Namespace Skip Auto-Propagation

**Symptom**: Namespace not auto-propagated to member clusters.

**Check**: Does the namespace have the label `namespace.karmada.io/skip-auto-propagation: "true"`?

If yes, the namespace controller will skip propagating it. Remove the label to re-enable auto-propagation.

## Diagnostic Flow

```
1. Check resource exists in control plane
     ↓ No → Resource was not created or was deleted
2. Check matching policy exists
     ↓ No → Create PropagationPolicy or ClusterPropagationPolicy
3. Check ResourceBinding created
     ↓ No → Policy selector mismatch; check apiVersion, kind, name, namespace, labels
4. Check ResourceBinding scheduled (spec.clusters populated)
     ↓ No → Check conditions for reason; adjust placement constraints
5. Check Work created for each target cluster
     ↓ No → Check suspension, execution controller logs
6. Check Work Applied condition
     ↓ No → Check conflict resolution, RBAC, agent connectivity
7. Check resource on member cluster
     ↓ No → Agent sync failure; check agent logs
```

## Common Mistakes

1. **Selector mismatch**: `apiVersion: v1` vs `apiVersion: apps/v1` for Deployment.
2. **Scope mismatch**: Using PropagationPolicy for cluster-scoped resources (use ClusterPropagationPolicy).
3. **Namespace mismatch**: PropagationPolicy in namespace A cannot match resources in namespace B.
4. **System namespace**: ClusterPropagationPolicy cannot match resources in `karmada-system`, `karmada-cluster`, or `karmada-es-*`.
5. **Toleration mismatch**: Cluster has taint but no toleration in policy placement.
6. **Conflict resolution**: Default `Abort` prevents overwriting existing resources on member clusters.
7. **Dispatching suspended**: `suspension.dispatching: true` blocks all propagation.
