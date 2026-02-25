# Karmada Prometheus PodMonitors

This directory contains PodMonitor resources for scraping metrics from Karmada components using the Prometheus Operator.

## Overview

These PodMonitors configure Prometheus to scrape metrics from all Karmada components running in the `karmada-system` namespace. They are designed to work with the kube-prometheus-stack Helm chart (identified by the `release: kps` label).

## Available PodMonitors

| Component | File | Port | TLS | Metrics Exposed |
|-----------|------|------|-----|-----------------|
| API Server | `apiserver-monitor.yaml` | https | ✅ | API server metrics, request rates, latency |
| Controller Manager | `controller-manager-monitor.yaml` | metrics | ❌ | Workqueue, reconciliation, cluster sync, propagation |
| Scheduler | `scheduler-monitor.yaml` | metrics | ❌ | Scheduling attempts, latency, queue status, plugin performance |
| Webhook | `webhook-monitor.yaml` | metrics | ❌ | Webhook requests, validation/mutation metrics |
| Descheduler | `descheduler-monitor.yaml` | metrics | ❌ | Descheduling metrics |
| Scheduler Estimator | `scheduler-estimator-monitor.yaml` | metrics | ❌ | Replica estimation metrics, algorithm latency |
| Metrics Adapter | `metrics-adapter-monitor.yaml` | metrics | ❌ | Custom metrics API, FederatedHPA metrics |
| Agent | `agent-monitor.yaml` | metrics | ❌ | Agent-specific metrics (when deployed) |

## Prerequisites

1. **Prometheus Operator**: The kube-prometheus-stack must be installed in the `monitoring` namespace
2. **TLS Certificate**: For API server scraping, run `hack/create-scrape-user.sh` to create the required TLS secret
3. **Karmada Components**: Components must be running in the `karmada-system` namespace with proper labels

## Installation

Apply all PodMonitors to the cluster:

```bash
kubectl apply -f prom/
```

Or using the convenience alias (after sourcing `.karmadarc`):

```bash
hc apply -f prom/
```

## Configuration

All PodMonitors are configured with:
- **Scrape interval**: 30 seconds
- **Scrape timeout**: 10 seconds
- **Namespace**: karmada-system
- **Label selector**: Matches `app: <component-name>`

### API Server TLS Configuration

The API server monitor requires TLS authentication because the metrics endpoint is protected. The required certificate is created by running:

```bash
hack/create-scrape-user.sh
```

This script creates a secret named `karmada-apiserver-scrape-tls` in the `monitoring` namespace containing:
- `ca.crt` - Certificate Authority
- `tls.crt` - Client certificate
- `tls.key` - Client private key

### Other Components

All other components expose metrics on an unsecured HTTP endpoint (typically port 8080, named `metrics` in the pod spec), so no TLS configuration is required.

## Verification

After applying the PodMonitors, verify they are scraping correctly:

1. **Check PodMonitor status**:
   ```bash
   kubectl get podmonitors -n monitoring
   ```

2. **Access Prometheus UI**:
   ```bash
   kubectl port-forward -n monitoring svc/kps-kube-prometheus-stack-prometheus 9090
   ```
   Then navigate to http://localhost:9090/targets to see all scrape targets.

3. **Test metrics availability**:
   ```bash
   # Query Prometheus for Karmada metrics
   curl -s http://localhost:9090/api/v1/label/__name__/values | jq '.data[]' | grep karmada
   ```

4. **Check specific component metrics**:
   ```promql
   # In Prometheus UI, run queries like:
   up{job="karmada-scheduler"}
   karmada_scheduler_schedule_attempts_total
   cluster_ready_state
   ```

## Troubleshooting

### No metrics appearing

**Check pod labels**:
```bash
kubectl get pods -n karmada-system --show-labels
```

Ensure pods have the correct `app` label matching the PodMonitor selector. For example:
- `app: karmada-scheduler`
- `app: karmada-controller-manager`

**Check metrics port**:
```bash
kubectl get pods -n karmada-system <pod-name> -o yaml | grep -A 5 "name: metrics"
```

Ensure the port named `metrics` exists and is correctly configured.

**Check PodMonitor label**:
```bash
kubectl get podmonitors -n monitoring -o yaml | grep "release: kps"
```

The `release: kps` label must match the Prometheus Operator's service monitor selector.

### API Server TLS errors

If you see TLS errors when scraping the API server:

1. Verify the secret exists:
   ```bash
   kubectl get secret -n monitoring karmada-apiserver-scrape-tls
   ```

2. Recreate the certificate:
   ```bash
   kubectl delete secret -n monitoring karmada-apiserver-scrape-tls
   hack/create-scrape-user.sh
   ```

3. Restart Prometheus:
   ```bash
   kubectl rollout restart statefulset -n monitoring kps-kube-prometheus-stack-prometheus
   ```

## Integration with Grafana Dashboards

These PodMonitors enable all metrics required for the Grafana dashboards located in:
```
docs/resources/administrator/grafana-dashboards/
```

After applying all PodMonitors, you should have complete observability coverage for:
- Karmada Overview
- Scheduling Performance
- Cluster Resources
- Controller Performance
- Propagation Pipeline
- Failover & Eviction
- Autoscaling

## Related Documentation

- [Karmada Observability Guide](../docs/administrator/monitoring/karmada-observability.md)
- [Karmada Metrics Reference](../docs/reference/instrumentation/metrics.md)
- [Grafana Dashboard README](../docs/resources/administrator/grafana-dashboards/README.md)
- [Setup Playbook](../playbook.md)

## Label Requirements

Each Karmada component must have the appropriate label for the PodMonitor to discover it:

```yaml
metadata:
  labels:
    app: karmada-<component-name>
```

If you're deploying Karmada components manually or via custom manifests, ensure these labels are present.
