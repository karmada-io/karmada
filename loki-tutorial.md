# Loki Logging Stack Setup Guide for kind

## Prerequisites

```bash
# Install kind if not already installed
# macOS
brew install kind

# Verify installation
kind version
```

## Step 1: Create a kind Cluster

```bash
# Create a kind cluster
kind create cluster --name loki-demo

# Verify cluster is running
kubectl cluster-info --context kind-loki-demo
```

## Step 2: Install Loki Stack using Helm

```bash
# Add Grafana Helm repository
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

# Create namespace for logging
kubectl create namespace logging

# Install Loki stack (includes Loki, Promtail, and Grafana)
helm install loki grafana/loki-stack \
  --namespace logging \
  --set grafana.enabled=true \
  --set prometheus.enabled=false \
  --set promtail.enabled=true \
  --set loki.persistence.enabled=false \
  --set loki.persistence.size=5Gi
```

## Step 3: Access Grafana

```bash
# Get Grafana admin password
kubectl get secret --namespace logging loki-grafana \
  -o jsonpath="{.data.admin-password}" | base64 --decode ; echo

# Port forward Grafana
kubectl port-forward --namespace logging service/loki-grafana 3000:80
```

Access Grafana at `http://localhost:3000`
- Username: `admin`
- Password: (from command above)

## Step 4: Configure Grafana Data Source

Loki should already be configured as a data source. To verify:

1. Go to Configuration → Data Sources
2. You should see Loki with URL: `http://loki:3100`

## Step 5: Deploy Sample Application (for testing)

```bash
# Create a sample deployment that generates logs
kubectl create deployment nginx --image=nginx --replicas=2
kubectl expose deployment nginx --port=80

# Generate some logs
kubectl run curl-test --image=curlimages/curl --rm -it --restart=Never \
  -- curl http://nginx
```

## Step 6: Query Logs in Grafana

1. Go to Explore (compass icon)
2. Select Loki data source
3. Try these LogQL queries:

```logql
# All logs
{namespace="default"}

# Logs from nginx pods
{namespace="default", app="nginx"}

# Search for specific text
{namespace="default"} |= "error"

# Count log lines
count_over_time({namespace="default"}[5m])
```

## Alternative: Install Loki Only (without Helm)

If you prefer to install components separately:

```bash
# Install Loki
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: logging
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: loki-config
  namespace: logging
data:
  loki.yaml: |
    auth_enabled: false
    server:
      http_listen_port: 3100
    ingester:
      lifecycler:
        ring:
          kvstore:
            store: inmemory
          replication_factor: 1
      chunk_idle_period: 5m
      chunk_retain_period: 30s
    schema_config:
      configs:
      - from: 2020-05-15
        store: boltdb
        object_store: filesystem
        schema: v11
        index:
          prefix: index_
          period: 168h
    storage_config:
      boltdb:
        directory: /tmp/loki/index
      filesystem:
        directory: /tmp/loki/chunks
    limits_config:
      enforce_metric_name: false
      reject_old_samples: true
      reject_old_samples_max_age: 168h
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki
  namespace: logging
spec:
  replicas: 1
  selector:
    matchLabels:
      app: loki
  template:
    metadata:
      labels:
        app: loki
    spec:
      containers:
      - name: loki
        image: grafana/loki:2.9.3
        args:
        - -config.file=/etc/loki/loki.yaml
        ports:
        - containerPort: 3100
          name: http
        volumeMounts:
        - name: config
          mountPath: /etc/loki
        - name: storage
          mountPath: /tmp/loki
      volumes:
      - name: config
        configMap:
          name: loki-config
      - name: storage
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: loki
  namespace: logging
spec:
  ports:
  - port: 3100
    targetPort: 3100
    name: http
  selector:
    app: loki
EOF

# Install Promtail
kubectl apply -f https://raw.githubusercontent.com/grafana/loki/v2.9.3/clients/cmd/promtail/promtail-kubernetes.yaml
```

## Step 7: Verify Installation

```bash
# Check Loki pods
kubectl get pods -n logging

# Check Loki logs
kubectl logs -n logging -l app=loki

# Check Promtail logs
kubectl logs -n logging -l app.kubernetes.io/name=promtail
```

## Useful LogQL Query Examples

```logql
# All logs from a specific pod
{pod="nginx-xxx"}

# Logs from specific namespace
{namespace="kube-system"}

# Filter by container
{container="nginx"}

# Pattern matching
{namespace="default"} |= "error" != "timeout"

# JSON parsing
{app="my-app"} | json | level="error"

# Metrics from logs
rate({namespace="default"}[5m])
```

## Cleanup

```bash
# Delete the kind cluster
kind delete cluster --name loki-demo
```

## Tips

- **Persistence**: For production, enable persistence in Loki to store logs long-term
- **Resource limits**: Set appropriate resource requests/limits based on log volume
- **Retention**: Configure log retention policies in Loki config
- **Grafana dashboards**: Import pre-built dashboards for Kubernetes logs
- **Labels**: Keep label cardinality low - don't use high-cardinality values as labels

This setup gives you a complete logging stack with Loki collecting logs from all pods in your kind cluster!
