# Loki Crash Course: A Pragmatic Guide

This guide covers the most useful Loki concepts and practical patterns for querying logs effectively.

---

## Table of Contents

1. [Core Concepts](#core-concepts)
2. [LogQL Basics](#logql-basics)
3. [Label Selectors](#label-selectors)
4. [Log Filtering](#log-filtering)
5. [Parsing Structured Logs](#parsing-structured-logs)
6. [Aggregation & Metrics from Logs](#aggregation--metrics-from-logs)
7. [Practical Query Patterns](#practical-query-patterns)
8. [Performance Tips](#performance-tips)

---

## Core Concepts

### What is Loki?

Loki is a **log aggregation system** designed to be cost-effective and easy to operate. Think of it as "Prometheus for logs."

**Key Design Principles:**
- **Labels, not full-text indexing** - Only metadata is indexed, not log content
- **Logs are stored as compressed chunks** - Similar to how Prometheus stores time-series data
- **Query logs like metrics** - Uses a query language (LogQL) inspired by PromQL

### Architecture

```
┌─────────────┐
│  Promtail   │ ──┐
└─────────────┘   │
                  │   Push logs
┌─────────────┐   │   ────────►   ┌─────────┐      ┌──────────┐
│  Promtail   │ ──┤                │  Loki   │ ◄──► │ Storage  │
└─────────────┘   │                └─────────┘      └──────────┘
                  │                     ▲
┌─────────────┐   │                     │
│  Promtail   │ ──┘                     │ Query
└─────────────┘                         │
                                        │
                                   ┌─────────┐
                                   │ Grafana │
                                   └─────────┘
```

**Components:**
- **Loki** - Log storage and query engine
- **Promtail** - Agent that collects logs from pods and ships to Loki
- **Grafana** - UI for querying and visualizing logs

### Labels vs Log Lines

**Labels** (indexed):
- Key-value pairs attached to log streams
- Examples: `namespace`, `pod`, `container`, `app`, `job`
- Used to identify and select log streams
- **Keep cardinality low** - Don't use high-cardinality values as labels

**Log Lines** (not indexed):
- The actual log content
- Searched using filters and parsers
- Can be structured (JSON) or unstructured (plain text)

**Example:**
```
Labels: {namespace="karmada-system", app="karmada-scheduler", pod="karmada-scheduler-abc123"}
Log Line: {"level":"info","ts":"2025-01-30T12:34:56Z","msg":"Successfully scheduled deployment","deployment":"my-app"}
```

---

## LogQL Basics

LogQL has two types of queries:

### 1. Log Queries (Return log lines)

```logql
{namespace="karmada-system"}
```

Returns raw log lines from all pods in `karmada-system` namespace.

### 2. Metric Queries (Return time-series data)

```logql
rate({namespace="karmada-system"}[5m])
```

Returns the rate of log lines per second over the last 5 minutes.

### Query Structure

```
{label selectors} | filter | parser | filter | aggregation
```

**Pipeline operators:**
- `|` - Pipe operator, chains operations left to right
- Each stage processes output from previous stage

---

## Label Selectors

Label selectors filter log streams **before** retrieving logs. Always start queries with label selectors.

### Operators

| Operator | Meaning | Example |
|----------|---------|---------|
| `=` | Exact match | `{app="karmada-scheduler"}` |
| `!=` | Not equal | `{namespace!="kube-system"}` |
| `=~` | Regex match | `{pod=~"karmada-.*"}` |
| `!~` | Regex not match | `{app!~"kube-.*"}` |

### Examples

**Single label:**
```logql
{namespace="karmada-system"}
```

**Multiple labels (AND):**
```logql
{namespace="karmada-system", app="karmada-controller-manager"}
```

**Regex matching:**
```logql
{namespace="karmada-system", app=~"karmada-controller-.*|karmada-scheduler"}
```

**Exclude namespace:**
```logql
{namespace!="kube-system"}
```

---

## Log Filtering

After selecting streams with labels, filter log lines by content.

### Filter Operators

| Operator | Meaning | Example |
|----------|---------|---------|
| `\|=` | Contains (case-sensitive) | `\|= "error"` |
| `!=` | Does not contain | `!= "healthz"` |
| `\|~` | Regex match | `\|~ "error\|warning"` |
| `!~` | Regex not match | `!~ "debug\|trace"` |

### Examples

**Find errors:**
```logql
{namespace="karmada-system"} |= "error"
```

**Exclude health checks:**
```logql
{namespace="karmada-system"} != "healthz"
```

**Case-insensitive search:**
```logql
{namespace="karmada-system"} |~ "(?i)error"
```

**Multiple filters (chained):**
```logql
{namespace="karmada-system"} |= "reconcile" |= "cluster" != "healthz"
```

---

## Parsing Structured Logs

Karmada components output JSON logs. Parse them to filter and extract fields.

### JSON Parser

Use `| json` to automatically parse JSON logs and extract fields as labels.

**Example log line:**
```json
{"level":"error","ts":"2025-01-30T12:34:56Z","msg":"Failed to sync cluster","cluster":"member1","error":"connection timeout"}
```

**Query:**
```logql
{namespace="karmada-system"} | json
```

Now you can filter by JSON fields:
```logql
{namespace="karmada-system"} | json | level="error"
```

### Extracting Specific Fields

**Filter by multiple JSON fields:**
```logql
{namespace="karmada-system"}
  | json
  | level="error"
  | cluster="member1"
```

**Regex on JSON field:**
```logql
{namespace="karmada-system"}
  | json
  | msg =~ ".*timeout.*"
```

**Extract and rename fields:**
```logql
{namespace="karmada-system"}
  | json level, msg, cluster
  | level="error"
```

### Other Parsers

**Pattern parser (for unstructured logs):**
```logql
{app="nginx"} | pattern "<ip> - - <_> \"<method> <path> <_>\" <status> <_>"
```

**Logfmt parser (key=value format):**
```logql
{app="app"} | logfmt | level="error"
```

**Regex parser:**
```logql
{app="app"} | regexp "level=(?P<level>\\w+)"
```

---

## Aggregation & Metrics from Logs

Convert logs into metrics for visualization and alerting.

### Range Vectors

Specify a time window using `[duration]`:
- `[5m]` - Last 5 minutes
- `[1h]` - Last 1 hour
- `[24h]` - Last 24 hours

### Count Operations

**Count log lines:**
```logql
count_over_time({namespace="karmada-system"}[5m])
```

**Count errors per component:**
```logql
sum by (app) (
  count_over_time({namespace="karmada-system"} | json | level="error" [5m])
)
```

**Error rate (logs per second):**
```logql
rate({namespace="karmada-system"} | json | level="error" [5m])
```

### Aggregation Functions

| Function | Description | Example |
|----------|-------------|---------|
| `sum` | Sum values | `sum(count_over_time(...))` |
| `avg` | Average | `avg(count_over_time(...))` |
| `min` / `max` | Min/Max | `max(count_over_time(...))` |
| `count` | Count series | `count(count_over_time(...))` |
| `topk` | Top K values | `topk(5, count_over_time(...))` |
| `bottomk` | Bottom K values | `bottomk(5, count_over_time(...))` |

### Group By Labels

**Errors by pod:**
```logql
sum by (pod) (
  count_over_time({namespace="karmada-system"} | json | level="error" [5m])
)
```

**Logs per component:**
```logql
sum by (app) (
  count_over_time({namespace="karmada-system"}[5m])
)
```

### Quantile and Percentiles

**Extract duration from logs and calculate p95:**
```logql
quantile_over_time(0.95,
  {namespace="karmada-system"}
    | json
    | unwrap duration [5m]
)
```

`unwrap` extracts numeric values from log lines for calculations.

---

## Practical Query Patterns

### Troubleshooting Errors

**All errors in the last hour:**
```logql
{namespace="karmada-system"} | json | level="error"
```

**Errors for specific cluster:**
```logql
{namespace="karmada-system"}
  | json
  | level="error"
  | cluster="member1"
```

**Top 5 pods with most errors:**
```logql
topk(5,
  sum by (pod) (
    count_over_time({namespace="karmada-system"} | json | level="error" [1h])
  )
)
```

### Investigating Specific Events

**Search for deployment reconciliation:**
```logql
{namespace="karmada-system", app="karmada-controller-manager"}
  |= "reconcile"
  |= "deployment"
```

**Find all timeout errors:**
```logql
{namespace="karmada-system"}
  | json
  | level="error"
  | msg =~ ".*timeout.*"
```

### Performance Analysis

**Count logs per second by component:**
```logql
sum by (app) (
  rate({namespace="karmada-system"}[5m])
)
```

**Components logging the most:**
```logql
topk(5,
  sum by (app) (
    count_over_time({namespace="karmada-system"}[1h])
  )
)
```

### Correlating Logs and Metrics

Use the same time range in both Prometheus and Loki queries to correlate:

**Loki (error rate):**
```logql
sum by (app) (
  rate({namespace="karmada-system"} | json | level="error" [5m])
)
```

**Prometheus (request rate):**
```promql
sum by (app) (
  rate(http_requests_total{namespace="karmada-system"}[5m])
)
```

### Live Tailing

In Grafana Explore, click **"Live"** button to stream logs in real-time. Great for:
- Watching deployments
- Debugging active issues
- Monitoring specific events

---

## Performance Tips

### 1. Always Start with Label Selectors

**Good:**
```logql
{namespace="karmada-system", app="karmada-scheduler"} |= "error"
```

**Bad:**
```logql
{namespace="karmada-system"} |= "error"
```

Narrow down streams first with labels before filtering content.

### 2. Avoid High-Cardinality Labels

**Bad labels** (too many unique values):
- User IDs
- Request IDs
- IP addresses
- Timestamps

**Good labels** (low cardinality):
- Namespace
- App name
- Environment (prod, staging)
- Pod name (reasonable)

### 3. Use Time Ranges Wisely

Shorter time ranges = faster queries. Start narrow, expand if needed.

**Fast:**
```logql
{namespace="karmada-system"} [5m]
```

**Slower:**
```logql
{namespace="karmada-system"} [24h]
```

### 4. Limit Results

Add `| limit 100` to cap number of log lines returned:
```logql
{namespace="karmada-system"} | json | level="error" | limit 100
```

### 5. Use Metric Queries for Dashboards

For dashboards and alerts, use metric queries instead of log queries:
```logql
sum by (app) (count_over_time({namespace="karmada-system"} | json | level="error" [5m]))
```

---

## Quick Reference Card

### Query Structure
```
{labels} | filter | parser | filter | aggregation
```

### Common Patterns

**View all logs:**
```logql
{namespace="karmada-system"}
```

**Errors only:**
```logql
{namespace="karmada-system"} | json | level="error"
```

**Search text:**
```logql
{namespace="karmada-system"} |= "cluster"
```

**Count errors:**
```logql
sum by (app) (count_over_time({namespace="karmada-system"} | json | level="error" [5m]))
```

**Error rate:**
```logql
rate({namespace="karmada-system"} | json | level="error" [5m])
```

### Label Operators
- `=` exact match
- `!=` not equal
- `=~` regex
- `!~` negative regex

### Filter Operators
- `|=` contains
- `!=` not contains
- `|~` regex
- `!~` negative regex

### Parsers
- `| json` - Parse JSON logs
- `| logfmt` - Parse key=value logs
- `| pattern` - Pattern matching
- `| regexp` - Regex extraction

---

## Next Steps

1. **Practice in Grafana Explore** - Navigate to Grafana → Explore → Select Loki
2. **Build Dashboards** - Create panels with metric queries for monitoring
3. **Set Up Alerts** - Alert on error rates using Loki queries
4. **Read Official Docs** - [https://grafana.com/docs/loki/latest/](https://grafana.com/docs/loki/latest/)

---

**Pro Tip:** Use Grafana's query builder (toggle "Builder/Code" button) to construct queries visually, then switch to code view to see the generated LogQL. Great way to learn!
