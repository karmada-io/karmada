# Karmada Setup and Monitoring Playbook

This playbook guides you through setting up a **Karmada** instance and configuring **Prometheus**, **Loki**, and **Grafana** for comprehensive monitoring and log collection on the Karmada host cluster.

---

## 1. Set Up a Local Karmada Instance

Run the following script to deploy a local Karmada environment:

```bash
hack/local-down-karmada.sh && hack/local-up-karmada.sh
```

This will create a Karmada instance that is joined to a set of Kind member clusters.

---

## 2. Load Shell Convenience Aliases

Source convenience aliases:

```bash
source .karmadarc
```

This step enables short commands like `hc` for interacting with the Karmada host cluster.

---

## 3. Install Kube-Prometheus-Stack in the Karmada Host Cluster

Add the Prometheus Helm repository and install the monitoring stack into the `monitoring` namespace:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm upgrade --install kps prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --kubeconfig ~/.kube/karmada.config \
  --kube-context karmada-host \
  -f monitoring-values.yaml
```

This installs Prometheus, Grafana, and related components for cluster observability. The `monitoring-values.yaml` file configures Grafana datasources properly to avoid conflicts.

---

## 4. Install Loki Stack for Log Collection

Add the Grafana Helm repository and install the Loki stack into the `monitoring` namespace:

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm upgrade --install loki grafana/loki-stack \
  --namespace monitoring \
  --kubeconfig ~/.kube/karmada.config \
  --kube-context karmada-host \
  --set promtail.enabled=true \
  --set loki.persistence.enabled=true \
  --set loki.persistence.size=10Gi \
  --set grafana.enabled=false
```

This installs Loki (log aggregation system) and Promtail (log collection agent) for centralized log management. Promtail automatically collects logs from all pods in the cluster and sends them to Loki.

---

## 5. Create a Certificate for Prometheus to Scrape the Karmada API Server

Generate the required TLS certificate for secure scraping:

```bash
hack/create-scrape-user.sh
```

This ensures Prometheus can securely collect metrics from the Karmada API server, whose metrics endpoint is protected.

---

## 6. Create PodMonitors

Apply the `PodMonitor` CRs to enable Prometheus to scrape metrics from the Karmada components:

```bash
hc apply -f prom
```

---

## 7. Retrieve Grafana Admin Credentials

Obtain the Grafana administrator password from the Kubernetes secret:

```bash
hc --namespace monitoring get secrets kps-grafana -o jsonpath="{.data.admin-password}" | base64 --decode
```

**Default credentials:**
- **Username:** `admin`
- **Password:** `prom-operator` (unless overridden)

---

## 8. Expose Grafana and Prometheus on the Host Network via Port Forwarding

Expose Grafana and Prometheus on the host network via port forwarding:

```bash
hc -n monitoring port-forward svc/kps-grafana 3010:80
```

```bash
hc -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090
```

Grafana will be available at [http://localhost:3010](http://localhost:3010), and Prometheus at [http://localhost:9090](http://localhost:9090).

---

## 9. Import Karmada Dashboards

Run the automated setup script to import all dashboards into Grafana:

```bash
hack/setup-grafana.sh
```

This script will:
- Create a "Karmada" folder in Grafana
- Import all dashboards from the `grafana/` directory into the Karmada folder

**Note:** Make sure the Grafana port-forward is running before executing this script. The Prometheus and Loki datasources are automatically configured via the `monitoring-values.yaml` file during Helm installation.

---

## 10. Querying Karmada Logs in Loki

Access Grafana at [http://localhost:3010](http://localhost:3010), navigate to **Explore**, select **Loki** as the data source, and run LogQL queries.

### Common Queries for Karmada Components

**View all logs from karmada-system namespace:**
```logql
{namespace="karmada-system"}
```

**Logs from specific component:**
```logql
{namespace="karmada-system", app="karmada-controller-manager"}
```

**Filter by log level (ERROR, WARN, INFO):**
```logql
{namespace="karmada-system"} | json | level="error"
```

**Search for specific text in logs:**
```logql
{namespace="karmada-system"} |= "reconcile"
```

**Exclude certain patterns:**
```logql
{namespace="karmada-system"} != "healthz"
```

**Multiple components (controller-manager OR scheduler):**
```logql
{namespace="karmada-system", app=~"karmada-controller-manager|karmada-scheduler"}
```

**Parse JSON logs and filter by field:**
```logql
{namespace="karmada-system"} | json | msg =~ ".*cluster.*"
```

**Count errors per component (last 5 minutes):**
```logql
sum by (app) (count_over_time({namespace="karmada-system"} | json | level="error" [5m]))
```

**View logs for a specific time range:**
- Use the time picker in Grafana UI (top right)
- Or add `[5m]` for last 5 minutes, `[1h]` for last hour

### Available Karmada Components

- `karmada-apiserver`
- `karmada-controller-manager`
- `karmada-scheduler`
- `karmada-descheduler`
- `karmada-webhook`
- `karmada-aggregated-apiserver`
- `karmada-search`
- `karmada-metrics-adapter`
- `etcd`
- `kube-controller-manager`

**Tip:** All Karmada components use structured JSON logging. Use `| json` in queries to parse and filter by JSON fields like `level`, `msg`, `caller`, etc.

For a comprehensive Loki guide, see [docs/loki-guide.md](docs/loki-guide.md).

---

## 11. Deploy Event Exporter for Kubernetes Events

Deploy the event exporter to capture Kubernetes API events as structured JSON logs in Loki:

```bash
hack/deploy-event-exporter.sh
```

This will:
- Create a client certificate for the event exporter to access Karmada API server
- Deploy the event exporter with proper RBAC permissions
- Configure the exporter to output events as JSON logs

### Verify Event Collection

Check that events are being exported:

```bash
hc logs -n karmada-system -l app=event-exporter -f
```

You should see JSON-formatted Kubernetes events streaming.

### Query Events in Loki

Access Grafana → Explore → Select Loki, then run:

**All events:**
```logql
{namespace="karmada-system", app="event-exporter"}
```

**Warning events only:**
```logql
{namespace="karmada-system", app="event-exporter"} | json | type="Warning"
```

**Pod scheduling failures:**
```logql
{namespace="karmada-system", app="event-exporter"} | json | reason="FailedScheduling"
```

**Events for specific pod:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | kind="Pod"
  | name="karmada-controller-manager-abc123"
```

**Formatted event view:**
```logql
{namespace="karmada-system", app="event-exporter"}
  | json
  | line_format "{{.type}} | {{.reason}} | {{.kind}}/{{.name}} | {{.message}}"
```

### View Events Dashboard

Navigate to Grafana → Dashboards → Karmada folder → **Karmada Events** dashboard.

The dashboard includes:
- Events timeline by type (Normal/Warning)
- Event count statistics
- Events distribution by reason and object kind
- Top components generating events
- Recent events log stream

For a complete guide on working with Kubernetes events in Loki, see [docs/kubernetes-events-guide.md](docs/kubernetes-events-guide.md).

---

## 12. Generate Test Events for Dashboard Testing

To populate the Events dashboard with realistic test data, use the event generation script:

```bash
hack/generate-events.sh \
  --kubeconfig ~/.kube/karmada.config \
  --context karmada-apiserver \
  --namespace default
```

This script will continuously generate various Kubernetes events by creating, updating, scaling, and deleting resources in an infinite loop.

### What Events Are Generated

The script creates diverse events across multiple scenarios:

**Normal Events:**
- Deployment creation and scaling (ScalingReplicaSet, SuccessfulCreate)
- ConfigMap creation and updates
- Service creation
- Pod lifecycle events (Scheduled, Pulling, Pulled, Created, Started)
- Init container events

**Warning Events:**
- Image pull failures (FailedPull, ErrImagePull, BackOff)
- Invalid image references

**Deletion Events:**
- Resource cleanup events (Killing, DELETE)

### Options

```bash
# Generate events in a specific namespace
hack/generate-events.sh \
  --kubeconfig ~/.kube/karmada.config \
  --context karmada-apiserver \
  --namespace my-test-namespace

# Enable trace mode for debugging
hack/generate-events.sh \
  --kubeconfig ~/.kube/karmada.config \
  --context karmada-apiserver \
  --trace
```

### Usage Tips

- Each iteration takes approximately 40-50 seconds and generates dozens of events
- Run for 5-10 minutes to populate the dashboard with sufficient data
- Press `Ctrl+C` to stop event generation when you have enough data
- Monitor events in real-time in Grafana → Dashboards → Karmada Events

### Verify Events Are Being Generated

Check the event exporter logs to see events streaming:

```bash
hc logs -n karmada-system -l app=event-exporter -f
```

Or query Loki in Grafana:

```logql
{namespace="karmada-system", app="event-exporter"} | json
```

---

### ✅ Summary

You now have:
- A local Karmada instance running.
- Monitoring enabled via Prometheus and Grafana.
- Log collection enabled via Loki and Promtail.
- Kubernetes event export to Loki for long-term analysis.
- Secure metric collection and custom dashboards for observability.
- Centralized logging with LogQL queries for troubleshooting.
- Events dashboard for cluster health visualization.


```shell
 helm upgrade --install kps prometheus-community/kube-prometheus-stack --namespace monitoring  --create-namespace --kubeconfig ~/.kube/karmada.config --kube-context karmada-host -f monitoring-values.yaml
```