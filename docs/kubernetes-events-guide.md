# Kubernetes Events in Loki: Complete Guide

This guide covers how to collect, query, and visualize Kubernetes events from the Karmada API server using Loki.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Deployment](#deployment)
4. [Event Structure](#event-structure)
5. [Querying Events](#querying-events)
6. [Dashboard](#dashboard)
7. [Common Use Cases](#common-use-cases)
8. [Troubleshooting](#troubleshooting)

---

## Overview

**Kubernetes Events** are time-stamped records of state changes in the cluster. They provide insight into:
- Pod lifecycle (scheduled, created, started, killed)
- Resource issues (failed scheduling, image pull errors)
- Controller actions (scaling, updating, reconciliation)
- Configuration problems (invalid specs, permission issues)

By default, events:
- Are stored in etcd with a **1-hour TTL**
- Are not persisted long-term
- Are difficult to search and analyze at scale

**Solution:** Export events as structured JSON logs to Loki for:
- Long-term retention
- Fast searching with LogQL
- Correlation with application logs
- Visualization in Grafana dashboards

---

## Architecture

```
┌──────────────────┐
│  Karmada API     │
│     Server       │
│                  │
│  ┌────────────┐  │
│  │   Events   │  │
│  └────────────┘  │
└────────┬─────────┘
         │ Watch
         │
    ┌────▼────────────────┐
    │  Event Exporter     │
    │                     │
    │  - Watches events   │
    │  - Converts to JSON │
    │  - Logs to stdout   │
    └────┬────────────────┘
         │ stdout (JSON logs)
         │
    ┌────▼────────────┐
    │   Promtail      │
    │                 │
    │  - Reads logs   │
    │  - Adds labels  │
    │  - Pushes to    │
    │    Loki         │
    └────┬────────────┘
         │
    ┌────▼─────┐       ┌──────────┐
    │   Loki   │ ◄───► │ Grafana  │
    └──────────┘       └──────────┘
```

**Components:**

1. **Event Exporter** - Kubernetes controller that watches events and outputs them as JSON logs
2. **Promtail** - Collects logs from event-exporter pod and ships to Loki
3. **Loki** - Stores and indexes event logs
4. **Grafana** - Queries and visualizes events

---

## Deployment

### Prerequisites

- Karmada cluster running (via `hack/local-up-karmada.sh`)
- Loki stack installed (via playbook step 4)
- Grafana configured (via playbook step 9)

### Deploy Event Exporter

Run the deployment script:

```bash
hack/deploy-event-exporter.sh
```

This script will:
1. Create a client certificate for the event exporter
2. Generate a kubeconfig secret to access Karmada API server
3. Deploy the event exporter with RBAC permissions
4. Wait for the deployment to be ready

### Verify Deployment

Check that the event exporter is running:

```bash
hc get pods -n karmada-system -l app=event-exporter
```

View event logs:

```bash
hc logs -n karmada-system -l app=event-exporter -f
```

You should see JSON-formatted event data streaming.

---

## Event Structure

Events exported by the event-exporter have the following JSON structure:

```json
{
  "type": "Normal",
  "reason": "Scheduled",
  "message": "Successfully assigned karmada-system/karmada-controller-manager-abc123 to node1",
  "kind": "Pod",
  "name": "karmada-controller-manager-abc123",
  "namespace": "karmada-system",
  "component": "default-scheduler",
  "host": "",
  "involvedObjectAPIVersion": "v1",
  "count": "1",
  "firstTimestamp": "2025-01-30T12:34:56Z",
  "lastTimestamp": "2025-01-30T12:34:56Z"
}
```

### Key Fields

| Field | Description | Example Values |
|-------|-------------|----------------|
| `type` | Event type | `Normal`, `Warning` |
| `reason` | Short reason code | `Scheduled`, `Created`, `FailedScheduling`, `BackOff` |
| `message` | Human-readable description | Full event message |
| `kind` | Resource type | `Pod`, `Deployment`, `Service`, `Node` |
| `name` | Resource name | Name of the object that generated the event |
| `namespace` | Namespace | Namespace of the object |
| `component` | Component that generated event | `kubelet`, `default-scheduler`, `karmada-controller-manager` |
| `count` | Number of times event occurred | Integer count |
| `firstTimestamp` | First occurrence | ISO 8601 timestamp |
| `lastTimestamp` | Last occurrence | ISO 8601 timestamp |

---

## Querying Events

Access Grafana → Explore → Select Loki data source.

### Basic Queries

**All events:**
```logql
{namespace="karmada-system", app="event-exporter"}
```

**Events in the last 5 minutes:**
```logql
{namespace="karmada-system", app="event-exporter"} [5m]
```

### Filter by Event Type

**Warning events only:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | type="Warning"
```

**Normal events only:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | type="Normal"
```

### Filter by Reason

**Failed scheduling:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | reason="FailedScheduling"
```

**Image pull errors:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | reason=~"Failed.*|ErrImagePull|ImagePullBackOff"
```

**Pod backoff (crash loop):**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | reason="BackOff"
```

### Filter by Object Kind

**Pod events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Pod"
```

**Deployment events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Deployment"
```

**Service events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Service"
```

### Filter by Specific Resource

**Events for specific pod:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Pod"
  | name="karmada-controller-manager-abc123"
```

**Events for all controller-manager pods:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Pod"
  | name=~"karmada-controller-manager-.*"
```

### Filter by Component

**Scheduler events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | component="default-scheduler"
```

**Controller events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | component=~".*controller.*"
```

### Search by Message Content

**Search for specific text in message:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | message =~ ".*timeout.*"
```

**Search for cluster-related events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | message =~ ".*cluster.*"
```

### Formatted Output

**Custom log line format:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | line_format "{{.type}} | {{.reason}} | {{.kind}}/{{.name}} | {{.message}}"
```

Output:
```
Warning | FailedScheduling | Pod/my-app-123 | 0/3 nodes are available
Normal | Scheduled | Pod/my-app-123 | Successfully assigned to node1
```

### Aggregations

**Count events by type:**
```logql
sum by (type) (count_over_time({namespace="karmada-system", app="event-exporter"} | json [5m]))
```

**Count warning events:**
```logql
sum(count_over_time({namespace="karmada-system", app="event-exporter"} | json | type="Warning" [5m]))
```

**Top 10 event reasons:**
```logql
topk(10, sum by (reason) (count_over_time({namespace="karmada-system", app="event-exporter"} | json [1h])))
```

**Event rate per second:**
```logql
rate({namespace="karmada-system", app="event-exporter"} | json [5m])
```

**Events by component:**
```logql
sum by (component) (count_over_time({namespace="karmada-system", app="event-exporter"} | json [1h]))
```

**Events by object kind:**
```logql
sum by (kind) (count_over_time({namespace="karmada-system", app="event-exporter"} | json [1h]))
```

---

## Dashboard

The Karmada Events dashboard (`grafana/karmada-events-dashboard.json`) provides:

### Panels

1. **Events Over Time by Type** - Time series showing Normal vs Warning events
2. **Warning Events** - Total count of warning events
3. **Normal Events** - Total count of normal events
4. **Total Events** - Overall event count
5. **Unique Object Kinds** - Number of different resource types generating events
6. **Events by Reason** - Pie chart of event reasons
7. **Events by Object Kind** - Pie chart of resource types
8. **Top 10 Components** - Components generating the most events
9. **Recent Events** - Live log stream of recent events

### Import Dashboard

The dashboard is automatically imported when you run `hack/setup-grafana.sh`.

To manually import:
1. Go to Grafana → Dashboards → Import
2. Upload `grafana/karmada-events-dashboard.json`
3. Select "Loki" as the data source
4. Click Import

---

## Common Use Cases

### 1. Debugging Pod Startup Issues

**Find why pod won't start:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Pod"
  | name="my-pod-name"
  | type="Warning"
```

Common reasons:
- `FailedScheduling` - No nodes available
- `ErrImagePull` - Can't pull container image
- `BackOff` - Container keeps crashing
- `FailedMount` - Volume mount issues

### 2. Monitoring Deployments

**Watch deployment rollout:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Deployment"
  | name="my-deployment"
```

Look for:
- `ScalingReplicaSet` - Deployment is scaling
- `ProgressDeadlineExceeded` - Rollout failed

### 3. Investigating Cluster Issues

**Find all warnings in last hour:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | type="Warning"
```

**High event rate (possible issue):**
```logql
sum(rate({namespace="karmada-system", app="event-exporter"} | json [5m])) > 10
```

### 4. Resource Quota Issues

**Find quota-related events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | message =~ ".*quota.*|.*exceeded.*"
```

### 5. Scheduling Problems

**All scheduling failures:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | reason="FailedScheduling"
```

### 6. Controller Reconciliation

**Controller manager events:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | component="karmada-controller-manager"
```

### 7. Alerting on Events

Create Loki alert rules for:

**Too many warnings:**
```logql
sum(rate({namespace="karmada-system", app="event-exporter"} | json | type="Warning" [5m])) > 1
```

**Image pull failures:**
```logql
sum(count_over_time({namespace="karmada-system", app="event-exporter"} | json | reason=~".*ImagePull.*" [5m])) > 0
```

**Pod crash loops:**
```logql
sum(count_over_time({namespace="karmada-system", app="event-exporter"} | json | reason="BackOff" [5m])) > 5
```

---

## Troubleshooting

### Event Exporter Not Sending Events

**Check if pod is running:**
```bash
hc get pods -n karmada-system -l app=event-exporter
```

**Check logs:**
```bash
hc logs -n karmada-system -l app=event-exporter
```

**Common issues:**
- **RBAC permissions** - Event exporter needs ClusterRole to list/watch events
- **Kubeconfig** - Secret must have valid Karmada API server credentials
- **API server connectivity** - Event exporter must reach Karmada API server

### No Events in Loki

**Check if Promtail is scraping event-exporter logs:**
```bash
hc logs -n monitoring -l app.kubernetes.io/name=promtail | grep event-exporter
```

**Verify Promtail configuration includes karmada-system namespace:**
```bash
hc get configmap -n monitoring loki-promtail -o yaml
```

**Test Loki query:**
```logql
{namespace="karmada-system", app="event-exporter"}
```

### Events Not Parsing Correctly

**Raw logs (without JSON parsing):**
```logql
{namespace="karmada-system", app="event-exporter"}
```

If logs aren't valid JSON, check event-exporter config:
```bash
hc get configmap -n karmada-system event-exporter-cfg -o yaml
```

### High Event Volume

If too many events are being generated:

**Identify noisy components:**
```logql
topk(5, sum by (component, reason) (count_over_time({namespace="karmada-system", app="event-exporter"} | json [5m])))
```

**Filter out noisy events in event-exporter config** by adding route filters.

---

## Best Practices

1. **Retention** - Configure Loki retention based on compliance needs (default: 30 days)
2. **Alerting** - Alert on critical event patterns (BackOff, FailedScheduling)
3. **Dashboard** - Pin the events dashboard for quick cluster health checks
4. **Correlation** - View events alongside pod logs when troubleshooting
5. **Search efficiency** - Always use `| json` after label selector for fast parsing

---

## Next Steps

- **Explore the dashboard** - Navigate to Grafana → Karmada Events dashboard
- **Set up alerts** - Create alert rules for critical event patterns
- **Correlate with logs** - Use split view in Grafana to see events + logs side-by-side
- **Build custom views** - Create dashboards for specific namespaces or components

---

## Reference

### Event Types

| Type | Description |
|------|-------------|
| `Normal` | Informational events about normal operations |
| `Warning` | Events that may indicate problems |

### Common Event Reasons

| Reason | Type | Description |
|--------|------|-------------|
| `Scheduled` | Normal | Pod assigned to node |
| `Pulled` | Normal | Container image pulled |
| `Created` | Normal | Container created |
| `Started` | Normal | Container started |
| `Killing` | Normal | Container being terminated |
| `FailedScheduling` | Warning | No nodes available for pod |
| `BackOff` | Warning | Container crash loop |
| `ErrImagePull` | Warning | Failed to pull image |
| `ImagePullBackOff` | Warning | Backing off after image pull failure |
| `FailedMount` | Warning | Volume mount failed |
| `Unhealthy` | Warning | Liveness/readiness probe failed |

### Useful LogQL Patterns

```logql
# Events timeline
{namespace="karmada-system", app="event-exporter"} | json

# Only warnings
{namespace="karmada-system", app="event-exporter"} | json | type="Warning"

# Specific resource
{namespace="karmada-system", app="event-exporter"} | json | kind="Pod" | name="my-pod"

# Search message
{namespace="karmada-system", app="event-exporter"} | json | message =~ ".*error.*"

# Count by reason
sum by (reason) (count_over_time({namespace="karmada-system", app="event-exporter"} | json [5m]))

# Event rate
rate({namespace="karmada-system", app="event-exporter"} | json [5m])
```

---

**Pro Tip:** Use Grafana's "Live" mode in Explore to watch events stream in real-time while deploying or troubleshooting. It's like `kubectl get events --watch` but with powerful filtering and retention!
