# Fluent Bit to Loki Setup Guide for kind

## Overview

This guide demonstrates how to set up Fluent Bit in a kind cluster to ship logs to Loki with enriched labels. Fluent Bit is a lightweight, high-performance log forwarder that pairs perfectly with Loki's label-based indexing approach.

**Key Benefits:**
- Lightweight collection (Fluent Bit uses ~1-10MB memory per node)
- Cost-effective storage (Loki only indexes labels, not content)
- Rich label enrichment for powerful log filtering
- Unified observability with Grafana

## Prerequisites

```bash
# Install kind if not already installed
# macOS
brew install kind

# Install helm
brew install helm

# Verify installations
kind version
helm version
```

## Step 1: Create a kind Cluster

```bash
# Create a kind cluster
kind create cluster --name fluent-loki

# Verify cluster is running
kubectl cluster-info --context kind-fluent-loki
kubectl get nodes
```

## Step 2: Install Loki and Grafana

```bash
# Add Grafana Helm repository
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

# Create namespace for logging
kubectl create namespace logging

# Install Loki
helm install loki grafana/loki \
  --namespace logging \
  --set loki.auth_enabled=false \
  --set loki.commonConfig.replication_factor=1 \
  --set singleBinary.replicas=1

# Install Grafana
helm install grafana grafana/grafana \
  --namespace logging \
  --set persistence.enabled=false \
  --set adminPassword=admin

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=loki -n logging --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=grafana -n logging --timeout=300s
```

## Step 3: Install Fluent Bit with Label Enrichment

### Quick Install (Helm with Basic Labels)

```bash
# Add Fluent Helm repository
helm repo add fluent https://fluent.github.io/helm-charts
helm repo update

# Install Fluent Bit with Loki output
helm install fluent-bit fluent/fluent-bit \
  --namespace logging \
  --set config.outputs="[OUTPUT]\n    Name loki\n    Match kube.*\n    Host loki-gateway\n    Port 80\n    Labels job=fluentbit, cluster=kind\n    Auto_Kubernetes_Labels on\n"

kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=fluent-bit -n logging --timeout=300s
```

### Advanced Install (Full Label Enrichment)

For maximum control over label enrichment, use this manual configuration:

```bash
# Create namespace
kubectl create namespace logging

# Create ConfigMap with enriched label configuration
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
  namespace: logging
  labels:
    app: fluent-bit
data:
  fluent-bit.conf: |
    [SERVICE]
        Daemon Off
        Flush 1
        Log_Level info
        Parsers_File parsers.conf
        Parsers_File custom_parsers.conf
        HTTP_Server On
        HTTP_Listen 0.0.0.0
        HTTP_Port 2020
        Health_Check On

    [INPUT]
        Name tail
        Path /var/log/containers/*.log
        multiline.parser docker, cri
        Tag kube.*
        Mem_Buf_Limit 5MB
        Skip_Long_Lines On
        Refresh_Interval 5

    [INPUT]
        Name systemd
        Tag host.*
        Systemd_Filter _SYSTEMD_UNIT=kubelet.service
        Read_From_Tail On

    # Kubernetes metadata enrichment
    [FILTER]
        Name kubernetes
        Match kube.*
        Kube_URL https://kubernetes.default.svc:443
        Kube_CA_File /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        Kube_Token_File /var/run/secrets/kubernetes.io/serviceaccount/token
        Kube_Tag_Prefix kube.var.log.containers.
        Merge_Log On
        Keep_Log Off
        K8S-Logging.Parser On
        K8S-Logging.Exclude On
        Labels On
        Annotations On
        Buffer_Size 256KB

    # Parse JSON logs
    [FILTER]
        Name parser
        Match kube.*
        Key_Name log
        Parser json
        Reserve_Data On
        Preserve_Key On

    # Add custom labels for Loki
    [FILTER]
        Name modify
        Match kube.*
        Add cluster kind-fluent-loki
        Add environment dev
        Add region local
        Add log_processor fluent-bit

    # Extract log level from JSON if present
    [FILTER]
        Name nest
        Match kube.*
        Operation lift
        Nested_under kubernetes
        Add_prefix k8s_

    # Record modifier to add node information
    [FILTER]
        Name record_modifier
        Match kube.*
        Record hostname \${HOSTNAME}
        Record node_name \${NODE_NAME}

    # Grep filter - exclude noisy logs (optional)
    [FILTER]
        Name grep
        Match kube.*
        Exclude log (healthz|readyz|livez)

    # Lua script for custom label extraction (optional)
    [FILTER]
        Name lua
        Match kube.*
        script /fluent-bit/scripts/add_labels.lua
        call add_custom_labels

    # Nest Kubernetes metadata for cleaner log structure
    [FILTER]
        Name nest
        Match kube.*
        Operation nest
        Wildcard k8s_*
        Nest_under kubernetes
        Remove_prefix k8s_

    # Output to Loki with enriched labels
    [OUTPUT]
        Name loki
        Match kube.*
        Host loki-gateway
        Port 80
        Labels job=fluentbit, cluster=kind-fluent-loki, environment=dev
        Label_keys \$kubernetes['namespace_name'],\$kubernetes['pod_name'],\$kubernetes['container_name'],\$kubernetes['host'],\$stream,\$level
        Auto_Kubernetes_Labels on
        Drop_Single_Key off
        Line_format json

    # Output for systemd logs
    [OUTPUT]
        Name loki
        Match host.*
        Host loki-gateway
        Port 80
        Labels job=systemd, cluster=kind-fluent-loki, component=kubelet
        Line_format json

    # Optional: stdout for debugging
    # [OUTPUT]
    #     Name stdout
    #     Match *
    #     Format json_lines

  parsers.conf: |
    [PARSER]
        Name docker
        Format json
        Time_Key time
        Time_Format %Y-%m-%dT%H:%M:%S.%L%z
        Time_Keep On

    [PARSER]
        Name cri
        Format regex
        Regex ^(?<time>[^ ]+) (?<stream>stdout|stderr) (?<logtag>[^ ]*) (?<log>.*)$
        Time_Key time
        Time_Format %Y-%m-%dT%H:%M:%S.%L%z
        Time_Keep On

    [PARSER]
        Name syslog
        Format regex
        Regex ^\<(?<pri>[0-9]+)\>(?<time>[^ ]* {1,2}[^ ]* [^ ]*) (?<host>[^ ]*) (?<ident>[a-zA-Z0-9_\/\.\-]*)(?:\[(?<pid>[0-9]+)\])?(?:[^\:]*\:)? *(?<message>.*)$
        Time_Key time
        Time_Format %b %d %H:%M:%S

  custom_parsers.conf: |
    [PARSER]
        Name json
        Format json
        Time_Key time
        Time_Format %Y-%m-%dT%H:%M:%S.%L%z
        Time_Keep On

    [PARSER]
        Name json_with_level
        Format json
        Time_Key timestamp
        Time_Format %Y-%m-%dT%H:%M:%S.%L%z
        Time_Keep On

    # Parser for nginx access logs
    [PARSER]
        Name nginx
        Format regex
        Regex ^(?<remote>[^ ]*) (?<host>[^ ]*) (?<user>[^ ]*) \[(?<time>[^\]]*)\] "(?<method>\S+)(?: +(?<path>[^\"]*?)(?: +\S*)?)?" (?<code>[^ ]*) (?<size>[^ ]*)(?: "(?<referer>[^\"]*)" "(?<agent>[^\"]*)")?$
        Time_Key time
        Time_Format %d/%b/%Y:%H:%M:%S %z

    # Parser for common application log formats
    [PARSER]
        Name app_log
        Format regex
        Regex ^\[(?<time>[^\]]*)\] (?<level>[A-Z]+) (?<message>.*)$
        Time_Key time
        Time_Format %Y-%m-%d %H:%M:%S

  add_labels.lua: |
    function add_custom_labels(tag, timestamp, record)
        -- Extract log level from various common fields
        local level = record["level"] or record["severity"] or record["lvl"] or "info"
        record["level"] = string.lower(level)

        -- Add app label from kubernetes labels if present
        if record["kubernetes"] and record["kubernetes"]["labels"] then
            local labels = record["kubernetes"]["labels"]
            if labels["app"] then
                record["app"] = labels["app"]
            elseif labels["app.kubernetes.io/name"] then
                record["app"] = labels["app.kubernetes.io/name"]
            end

            -- Extract version/release
            if labels["version"] or labels["app.kubernetes.io/version"] then
                record["version"] = labels["version"] or labels["app.kubernetes.io/version"]
            end

            -- Extract component
            if labels["component"] or labels["app.kubernetes.io/component"] then
                record["component"] = labels["component"] or labels["app.kubernetes.io/component"]
            end
        end

        -- Classify log severity
        if record["level"] == "error" or record["level"] == "fatal" or record["level"] == "panic" then
            record["severity_class"] = "critical"
        elseif record["level"] == "warn" or record["level"] == "warning" then
            record["severity_class"] = "warning"
        else
            record["severity_class"] = "normal"
        end

        return 2, timestamp, record
    end
EOF

# Create RBAC for Fluent Bit
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: fluent-bit
  namespace: logging
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: fluent-bit-read
rules:
- apiGroups: [""]
  resources:
  - namespaces
  - pods
  - nodes
  - nodes/proxy
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: fluent-bit-read
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: fluent-bit-read
subjects:
- kind: ServiceAccount
  name: fluent-bit
  namespace: logging
EOF

# Deploy Fluent Bit DaemonSet
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluent-bit
  namespace: logging
  labels:
    app: fluent-bit
spec:
  selector:
    matchLabels:
      app: fluent-bit
  template:
    metadata:
      labels:
        app: fluent-bit
    spec:
      serviceAccountName: fluent-bit
      containers:
      - name: fluent-bit
        image: fluent/fluent-bit:2.2.2
        imagePullPolicy: IfNotPresent
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        ports:
        - name: http
          containerPort: 2020
          protocol: TCP
        resources:
          limits:
            cpu: 200m
            memory: 256Mi
          requests:
            cpu: 50m
            memory: 64Mi
        volumeMounts:
        - name: varlog
          mountPath: /var/log
          readOnly: true
        - name: varlibdockercontainers
          mountPath: /var/lib/docker/containers
          readOnly: true
        - name: fluent-bit-config
          mountPath: /fluent-bit/etc/
        - name: fluent-bit-scripts
          mountPath: /fluent-bit/scripts/
      volumes:
      - name: varlog
        hostPath:
          path: /var/log
      - name: varlibdockercontainers
        hostPath:
          path: /var/lib/docker/containers
      - name: fluent-bit-config
        configMap:
          name: fluent-bit-config
      - name: fluent-bit-scripts
        configMap:
          name: fluent-bit-config
          items:
          - key: add_labels.lua
            path: add_labels.lua
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      - operator: "Exists"
        effect: "NoExecute"
      - operator: "Exists"
        effect: "NoSchedule"
EOF

# Create Service for monitoring
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: fluent-bit
  namespace: logging
  labels:
    app: fluent-bit
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 2020
    targetPort: http
    protocol: TCP
  selector:
    app: fluent-bit
EOF

# Wait for Fluent Bit to be ready
kubectl wait --for=condition=ready pod -l app=fluent-bit -n logging --timeout=120s
```

## Step 4: Deploy Sample Applications with Labels

```bash
# Create demo namespace
kubectl create namespace demo

# Deploy nginx with labels
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-frontend
  namespace: demo
  labels:
    app: nginx
    component: frontend
    tier: web
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
      component: frontend
  template:
    metadata:
      labels:
        app: nginx
        component: frontend
        tier: web
        version: "1.0"
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        env:
        - name: LOG_LEVEL
          value: "info"
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: demo
  labels:
    app: nginx
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
EOF

# Deploy backend service with different labels
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-backend
  namespace: demo
  labels:
    app: api
    component: backend
    tier: service
spec:
  replicas: 1
  selector:
    matchLabels:
      app: api
      component: backend
  template:
    metadata:
      labels:
        app: api
        component: backend
        tier: service
        version: "2.1.0"
    spec:
      containers:
      - name: api
        image: hashicorp/http-echo
        args:
        - "-text=API Response"
        ports:
        - containerPort: 5678
EOF

# Deploy structured log generator
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: log-generator
  namespace: demo
  labels:
    app: log-generator
    component: testing
spec:
  replicas: 1
  selector:
    matchLabels:
      app: log-generator
  template:
    metadata:
      labels:
        app: log-generator
        component: testing
        version: "1.0.0"
    spec:
      containers:
      - name: logger
        image: busybox
        command: ["/bin/sh"]
        args:
        - -c
        - |
          counter=0
          while true; do
            # Info log
            echo "{\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"info\",\"message\":\"Processing request\",\"request_id\":\"req-$counter\",\"user\":\"user$((counter % 10))\",\"duration_ms\":$((RANDOM % 1000))}"
            sleep 2

            # Warning log
            if [ $((counter % 5)) -eq 0 ]; then
              echo "{\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"warning\",\"message\":\"High latency detected\",\"request_id\":\"req-$counter\",\"latency_ms\":$((RANDOM % 5000 + 1000))}"
            fi
            sleep 1

            # Error log
            if [ $((counter % 10)) -eq 0 ]; then
              echo "{\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"error\",\"message\":\"Failed to connect to database\",\"request_id\":\"req-$counter\",\"error_code\":\"DB_CONN_FAILED\",\"retry_count\":3}"
            fi
            sleep 2

            counter=$((counter + 1))
          done
EOF
```

## Step 5: Configure Grafana and Query Logs

### Access Grafana

```bash
# Get Grafana password (if you didn't set it during install)
kubectl get secret -n logging grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo

# Port forward Grafana
kubectl port-forward -n logging svc/grafana 3000:80
```

Access Grafana at `http://localhost:3000`
- Username: `admin`
- Password: `admin` (or from command above)

### Add Loki Data Source

1. Go to **Configuration** → **Data Sources**
2. Click **Add data source**
3. Select **Loki**
4. Set URL: `http://loki-gateway:80`
5. Click **Save & Test**

### Query Examples Using Enriched Labels

Go to **Explore** and try these LogQL queries:

#### Basic Label-based Queries

```logql
# All logs from demo namespace
{namespace="demo"}

# Logs from specific app
{namespace="demo", app="nginx"}

# Logs by component
{namespace="demo", component="frontend"}

# Logs by cluster
{cluster="kind-fluent-loki"}

# Logs by environment
{environment="dev"}

# Logs from specific container
{namespace="demo", container="nginx"}
```

#### Advanced Queries with Multiple Labels

```logql
# Frontend logs only
{namespace="demo", component="frontend", tier="web"}

# Backend service logs
{namespace="demo", component="backend"}

# All logs from a specific version
{namespace="demo", version="1.0"}

# Logs from specific node
{node_name="kind-fluent-loki-control-plane"}

# Logs processed by fluent-bit
{log_processor="fluent-bit"}
```

#### Filtering Log Content

```logql
# Error logs only
{namespace="demo"} |= "error"

# Specific error code
{namespace="demo"} | json | error_code="DB_CONN_FAILED"

# High latency warnings
{namespace="demo"} | json | level="warning" | latency_ms > 2000

# Filter by JSON field
{namespace="demo", app="log-generator"} | json | user="user5"

# Regex pattern matching
{namespace="demo"} |~ "request.*failed"

# Exclude health checks
{namespace="demo"} != "healthz"
```

#### Metrics from Logs

```logql
# Rate of all logs
rate({namespace="demo"}[5m])

# Error rate
rate({namespace="demo"} |= "error" [5m])

# Count by app
sum by (app) (rate({namespace="demo"}[5m]))

# Count by level
sum by (level) (rate({namespace="demo"} | json [5m]))

# Top 10 users by log volume
topk(10, sum by (user) (rate({namespace="demo"} | json [1h])))

# Average request duration
avg_over_time({namespace="demo"} | json | unwrap duration_ms [5m])

# 95th percentile latency
quantile_over_time(0.95, {namespace="demo"} | json | unwrap latency_ms [5m])
```

#### Complex Aggregations

```logql
# Error rate by component
sum by (component) (
  rate({namespace="demo"} | json | level="error" [5m])
)

# Logs per second by namespace
sum by (namespace) (
  rate({cluster="kind-fluent-loki"}[1m])
)

# Request count by user
sum by (user) (
  count_over_time({namespace="demo", app="log-generator"} | json [1h])
)
```

## Understanding Label Enrichment

### Labels Added by Fluent Bit

The configuration adds multiple layers of labels:

#### 1. Static Labels (from OUTPUT config)
```yaml
Labels job=fluentbit, cluster=kind-fluent-loki, environment=dev
```
These are added to ALL logs sent to Loki.

#### 2. Kubernetes Labels (Auto_Kubernetes_Labels)
Automatically extracted from Kubernetes:
- `namespace` - Pod namespace
- `pod` - Pod name
- `container` - Container name
- `host` - Node hostname
- `stream` - stdout or stderr
- Plus any pod labels (app, component, version, etc.)

#### 3. Custom Labels (from FILTER modify)
```yaml
[FILTER]
    Name modify
    Add cluster kind-fluent-loki
    Add environment dev
    Add region local
    Add log_processor fluent-bit
```

#### 4. Dynamic Labels (from record_modifier)
```yaml
[FILTER]
    Name record_modifier
    Record hostname ${HOSTNAME}
    Record node_name ${NODE_NAME}
```

#### 5. Extracted Labels (from Lua script)
The Lua script extracts:
- `level` - Log level (info, warn, error)
- `app` - Application name
- `version` - Application version
- `component` - Application component
- `severity_class` - Categorized severity

### Label Cardinality Warning

**Important**: Loki performance depends on keeping label cardinality low.

❌ **BAD - High Cardinality Labels:**
```yaml
# Don't use these as labels in Loki output
- request_id (unique per request)
- user_id (potentially millions)
- timestamp (unique per log)
- ip_address (many unique values)
- session_id (unique per session)
```

✅ **GOOD - Low Cardinality Labels:**
```yaml
# Use these as labels
- namespace (limited number)
- app (limited number)
- environment (dev, staging, prod)
- cluster (limited number)
- level (info, warn, error, debug)
- component (frontend, backend, database)
- region (us-east, us-west, etc.)
```

**Best Practice**: Keep total unique label combinations under 10,000. Use log filtering (|=, |~, | json) for high-cardinality data instead of labels.

## Step 6: Verify Log Flow

```bash
# Check Fluent Bit is collecting logs
kubectl get pods -n logging -l app=fluent-bit

# View Fluent Bit logs
kubectl logs -n logging -l app=fluent-bit --tail=50

# Check for errors
kubectl logs -n logging -l app=fluent-bit | grep -i error

# Verify Loki is receiving data
kubectl logs -n logging -l app.kubernetes.io/name=loki --tail=50

# Check Fluent Bit metrics
kubectl port-forward -n logging svc/fluent-bit 2020:2020
curl http://localhost:2020/api/v1/metrics | grep output
```

## Advanced Label Enrichment Patterns

### Pattern 1: Environment-based Labeling

```yaml
[FILTER]
    Name modify
    Match kube.*
    Condition Key_Value_Equals kubernetes['namespace_name'] production
    Add environment production
    Add criticality high

[FILTER]
    Name modify
    Match kube.*
    Condition Key_Value_Equals kubernetes['namespace_name'] staging
    Add environment staging
    Add criticality medium
```

### Pattern 2: Service-based Labeling

```yaml
[FILTER]
    Name modify
    Match kube.*
    Condition Key_Value_Matches kubernetes['labels']['app'] ^(nginx|httpd)$
    Add service_type web

[FILTER]
    Name modify
    Match kube.*
    Condition Key_Value_Matches kubernetes['labels']['app'] ^(postgres|mysql|redis)$
    Add service_type database
```

### Pattern 3: Team Ownership Labels

```yaml
[FILTER]
    Name modify
    Match kube.*
    Condition Key_Value_Equals kubernetes['labels']['team'] platform
    Add team platform
    Add slack_channel platform-alerts

[FILTER]
    Name modify
    Match kube.*
    Condition Key_Value_Equals kubernetes['labels']['team'] backend
    Add team backend
    Add slack_channel backend-alerts
```

### Pattern 4: Multi-cluster Labeling

```yaml
[FILTER]
    Name modify
    Match *
    Add cluster_id ${CLUSTER_ID}
    Add cluster_region ${AWS_REGION}
    Add cluster_env ${ENVIRONMENT}
```

## Monitoring and Troubleshooting

### Monitor Fluent Bit Performance

```bash
# Port forward metrics endpoint
kubectl port-forward -n logging svc/fluent-bit 2020:2020

# Check input metrics
curl http://localhost:2020/api/v1/metrics | grep input

# Check output metrics
curl http://localhost:2020/api/v1/metrics | grep output

# Check for errors/retries
curl http://localhost:2020/api/v1/metrics | grep -E '(error|retry)'

# Prometheus format metrics
curl http://localhost:2020/api/v1/metrics/prometheus
```

### Common Issues and Solutions

#### 1. Labels Not Appearing in Loki

```bash
# Check Fluent Bit configuration
kubectl get configmap fluent-bit-config -n logging -o yaml

# Verify Kubernetes filter is working
kubectl logs -n logging -l app=fluent-bit | grep kubernetes

# Check Loki output configuration
kubectl logs -n logging -l app=fluent-bit | grep "loki"
```

**Solution**: Ensure `Auto_Kubernetes_Labels on` is set and RBAC permissions are correct.

#### 2. Too Many Labels (High Cardinality)

Check label cardinality in Grafana:
```logql
# See all label combinations
{cluster="kind-fluent-loki"}
```

**Solution**: Review Label_keys in OUTPUT section and remove high-cardinality labels.

#### 3. Missing Kubernetes Metadata

```bash
# Test RBAC permissions
kubectl auth can-i get pods --as=system:serviceaccount:logging:fluent-bit --all-namespaces
kubectl auth can-i list pods --as=system:serviceaccount:logging:fluent-bit --all-namespaces
```

**Solution**: Ensure ClusterRole has proper permissions.

#### 4. Loki Connection Issues

```bash
# Test connectivity from Fluent Bit pod
kubectl exec -n logging -it $(kubectl get pod -n logging -l app=fluent-bit -o jsonpath='{.items[0].metadata.name}') -- sh

# Inside the pod
wget -O- http://loki-gateway:80/ready
```

**Solution**: Verify Loki service name and port.

### Enable Debug Logging

```yaml
[SERVICE]
    Log_Level debug

[OUTPUT]
    Name loki
    Match *
    Host loki-gateway
    Port 80
    Labels job=fluentbit
    # Add this for debugging
    Log_Response_Payload On
```

## Grafana Dashboard Examples

### Create Dashboard for Error Tracking

1. Create new dashboard
2. Add panel with query:
```logql
sum by (namespace, app) (
  rate({cluster="kind-fluent-loki"} | json | level="error" [5m])
)
```
3. Set visualization to Time series
4. Add alert threshold

### Create Dashboard for Log Volume

```logql
sum by (namespace) (
  rate({cluster="kind-fluent-loki"}[1m])
)
```

### Create Dashboard for Top Talkers

```logql
topk(10, sum by (app, namespace) (
  rate({cluster="kind-fluent-loki"}[5m])
))
```

## Performance Tuning

### Optimize Fluent Bit for High Volume

```yaml
[SERVICE]
    Flush 5
    Grace 30

[INPUT]
    Name tail
    Path /var/log/containers/*.log
    Mem_Buf_Limit 10MB
    Skip_Long_Lines On
    Refresh_Interval 5
    Rotate_Wait 30

[OUTPUT]
    Name loki
    Match *
    Host loki-gateway
    Port 80
    Labels job=fluentbit
    Retry_Limit 3
    Workers 2
```

### Resource Limits

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

## Cleanup

```bash
# Delete demo namespace
kubectl delete namespace demo

# Uninstall Fluent Bit
kubectl delete daemonset fluent-bit -n logging
kubectl delete configmap fluent-bit-config -n logging
kubectl delete serviceaccount fluent-bit -n logging
kubectl delete service fluent-bit -n logging
kubectl delete clusterrolebinding fluent-bit-read
kubectl delete clusterrole fluent-bit-read

# Uninstall Loki and Grafana (if using Helm)
helm uninstall loki -n logging
helm uninstall grafana -n logging

# Delete namespace
kubectl delete namespace logging

# Delete kind cluster
kind delete cluster --name fluent-loki
```

## Best Practices Summary

1. **Label Strategy**
   - Keep label cardinality low (< 10,000 combinations)
   - Use static labels for environment, cluster, region
   - Use Kubernetes labels for namespace, app, component
   - Filter high-cardinality data in LogQL, don't use as labels

2. **Performance**
   - Set appropriate buffer sizes
   - Use Workers for high-volume outputs
   - Configure retry limits
   - Monitor Fluent Bit metrics

3. **Security**
   - Use RBAC with minimal permissions
   - Enable TLS for Loki communication in production
   - Don't log sensitive data
   - Use namespace isolation

4. **Operational**
   - Monitor Fluent Bit resource usage
   - Set up alerts for high error rates
   - Regular testing of log queries
   - Document custom label meanings

5. **Label Naming**
   - Use consistent naming conventions
   - Avoid special characters
   - Keep names short and meaningful
   - Document team-specific labels

## Useful Commands Reference

```bash
# View current configuration
kubectl get configmap fluent-bit-config -n logging -o yaml

# Update configuration
kubectl edit configmap fluent-bit-config -n logging
kubectl rollout restart daemonset/fluent-bit -n logging

# Check pod status
kubectl get pods -n logging -l app=fluent-bit -o wide

# View logs from all Fluent Bit pods
kubectl logs -n logging -l app=fluent-bit --all-containers=true -f

# Check resource usage
kubectl top pods -n logging -l app=fluent-bit

# Exec into Fluent Bit pod
kubectl exec -it -n logging $(kubectl get pod -n logging -l app=fluent-bit -o jsonpath='{.items[0].metadata.name}') -- sh

# Test Loki query API directly
kubectl port-forward -n logging svc/loki-gateway 3100:80
curl -G -s "http://localhost:3100/loki/api/v1/query" --data-urlencode 'query={namespace="demo"}' | jq
```

## Conclusion

This setup provides:
- ✅ Lightweight log collection with Fluent Bit
- ✅ Cost-effective storage with Loki
- ✅ Rich label enrichment for powerful filtering
- ✅ Unified observability with Grafana
- ✅ Low resource overhead
- ✅ Scalable architecture

The enriched labels enable precise log filtering and powerful analytics while maintaining Loki's performance benefits through low-cardinality label design.
