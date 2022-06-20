# Use Prometheus to monitor Karmada member clusters

[Prometheus](https://github.com/prometheus/prometheus), a [Cloud Native Computing Foundation](https://cncf.io/) project, is a system and service monitoring system. It collects metrics from configured targets at given intervals, evaluates rule expressions, displays the results, and can trigger alerts when specified conditions are observed.

This document gives an example to demonstrate how to use the `Prometheus` to monitor Karmada member clusters.

## Start up Karmada clusters
You just need to clone Karmada repo, and run the following script in Karmada directory.

```shell
hack/local-up-karmada.sh
```

## Start Prometheus

1. Create resource objects of Prometheus, the content is as follows.

   ```
   apiVersion: v1
   kind: Namespace
   metadata:
      name: monitor
      labels:
        name: monitor
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: prometheus
   rules:
   - apiGroups: [""]
     resources:
     - nodes
     - nodes/proxy
     - services
     - endpoints
     - pods
     verbs: ["get", "list", "watch"]
   - apiGroups:
     - extensions
     resources:
     - ingresses
     verbs: ["get", "list", "watch"]
   - nonResourceURLs: ["/metrics"]
     verbs: ["get"]
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: prometheus
     namespace: monitor
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: prometheus
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: prometheus
   subjects:
   - kind: ServiceAccount
     name: prometheus
     namespace: monitor
   ---
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: prometheus-config
     namespace: monitor
   data:
     prometheus.yml: |
       global:
         scrape_interval:     15s
         evaluation_interval: 15s
       scrape_configs:
       - job_name: 'kubernetes-apiservers'
         kubernetes_sd_configs:
         - role: endpoints
         scheme: https
         tls_config:
           ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
         bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
         relabel_configs:
         - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
           action: keep
           regex: default;kubernetes;https
       - job_name: 'kubernetes-nodes'
         kubernetes_sd_configs:
         - role: node
         scheme: https
         tls_config:
           ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
         bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
         relabel_configs:
         - action: labelmap
           regex: __meta_kubernetes_node_label_(.+)
         - target_label: __address__
           replacement: kubernetes.default.svc:443
         - source_labels: [__meta_kubernetes_node_name]
           regex: (.+)
           target_label: __metrics_path__
           replacement: /api/v1/nodes/${1}/proxy/metrics
       - job_name: 'kubernetes-cadvisor'
         kubernetes_sd_configs:
         - role: node
         scheme: https
         tls_config:
           ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
         bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
         relabel_configs:
         - action: labelmap
           regex: __meta_kubernetes_node_label_(.+)
         - target_label: __address__
           replacement: kubernetes.default.svc:443
         - source_labels: [__meta_kubernetes_node_name]
           regex: (.+)
           target_label: __metrics_path__
           replacement: /api/v1/nodes/${1}/proxy/metrics/cadvisor
       - job_name: 'kubernetes-service-endpoints'
         kubernetes_sd_configs:
         - role: endpoints
         relabel_configs:
         - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
           action: keep
           regex: true
         - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
           action: replace
           target_label: __scheme__
           regex: (https?)
         - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
           action: replace
           target_label: __metrics_path__
           regex: (.+)
         - source_labels: [__address__,__meta_kubernetes_service_annotation_prometheus_io_port]
           action: replace
           target_label: __address__
           regex: ([^:]+)(?::\d+)?;(\d+)
           replacement: $1:$2
         - action: labelmap
           regex: __meta_kubernetes_service_label_(.+)
         - source_labels: [__meta_kubernetes_namespace]
           action: replace
           target_label: kubernetes_namespace
         - source_labels: [__meta_kubernetes_service_name]
           action: replace
           target_label: kubernetes_name
       - job_name: 'kubernetes-services'
         kubernetes_sd_configs:
         - role: service
         metrics_path: /probe
         params:
           module: [http_2xx]
         relabel_configs:
         - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_probe]
           action: keep
           regex: true
         - source_labels: [__address__]
           target_label: __param_target
         - target_label: __address__
           replacement: blackbox-exporter.example.com:9115
         - source_labels: [__param_target]
           target_label: instance
         - action: labelmap
           regex: __meta_kubernetes_service_label_(.+)
         - source_labels: [__meta_kubernetes_namespace]
           target_label: kubernetes_namespace
         - source_labels: [__meta_kubernetes_service_name]
           target_label: kubernetes_name
       - job_name: 'kubernetes-ingresses'
         kubernetes_sd_configs:
         - role: ingress
         relabel_configs:
         - source_labels: [__meta_kubernetes_ingress_annotation_prometheus_io_probe]
           action: keep
           regex: true
         - source_labels: [__meta_kubernetes_ingress_scheme,__address__,__meta_kubernetes_ingress_path]
           regex: (.+);(.+);(.+)
           replacement: ${1}://${2}${3}
           target_label: __param_target
         - target_label: __address__
           replacement: blackbox-exporter.example.com:9115
         - source_labels: [__param_target]
           target_label: instance
         - action: labelmap
           regex: __meta_kubernetes_ingress_label_(.+)
         - source_labels: [__meta_kubernetes_namespace]
           target_label: kubernetes_namespace
         - source_labels: [__meta_kubernetes_ingress_name]
           target_label: kubernetes_name
       - job_name: 'kubernetes-pods'
         kubernetes_sd_configs:
         - role: pod
         relabel_configs:
         - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
           action: keep
           regex: true
         - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
           action: replace
           target_label: __metrics_path__
           regex: (.+)
         - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
           action: replace
           regex: ([^:]+)(?::\d+)?;(\d+)
           replacement: $1:$2
           target_label: __address__
         - action: labelmap
           regex: __meta_kubernetes_pod_label_(.+)
         - source_labels: [__meta_kubernetes_namespace]
           action: replace
           target_label: kubernetes_namespace
         - source_labels: [__meta_kubernetes_pod_name]
           action: replace
           target_label: kubernetes_pod_name
       - job_name: kube-state-metrics
         static_configs:
         - targets: ['kube-state-metrics.monitor.svc.cluster.local:8080']
   ---
   kind: Service
   apiVersion: v1
   metadata:
     labels:
       app: prometheus
     name: prometheus
     namespace: monitor
   spec:
     type: NodePort
     ports:
     - port: 9090
       targetPort: 9090
       nodePort: 30003
     selector:
       app: prometheus
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     labels:
       name: prometheus-deployment
     name: prometheus
     namespace: monitor
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: prometheus
     template:
       metadata:
         labels:
           app: prometheus
       spec:
         containers:
         - image: prom/prometheus
           imagePullPolicy: IfNotPresent
           name: prometheus
           command:
           - "/bin/prometheus"
           args:
           - "--config.file=/etc/prometheus/prometheus.yml"
           - "--storage.tsdb.path=/home/prometheus"
           - "--storage.tsdb.retention=168h"
           - "--web.enable-lifecycle"
           ports:
           - containerPort: 9090
             protocol: TCP
           volumeMounts:
           - mountPath: "/home/prometheus"
             name: data
           - mountPath: "/etc/prometheus"
             name: config-volume
           resources:
             requests:
               cpu: 100m
               memory: 256Mi
             limits:
               cpu: 500m
               memory: 3180Mi
         serviceAccountName: prometheus    
         securityContext:
           runAsUser: 0
         volumes:
         - name: data
           hostPath:
             path: "/data/prometheus/data"
         - name: config-volume
           configMap:
             name: prometheus-config
   ```

2. Run the below command to execute Karmada PropagationPolicy and ClusterPropagationPolicy.

   ```
   cat <<EOF | kubectl apply -f -
   apiVersion: policy.karmada.io/v1alpha1
   kind: PropagationPolicy
   metadata:
     name: prometheus-propagation
     namespace: monitor
   spec:
     resourceSelectors:
       - apiVersion: v1
         kind: Namespace
         name: monitor
       - apiVersion: v1
         kind: ServiceAccount
         name: prometheus
         namespace: monitor
       - apiVersion: v1
         kind: ConfigMap
         name: prometheus-config
         namespace: monitor
       - apiVersion: v1
         kind: Service
         name: prometheus
         namespace: monitor
       - apiVersion: apps/v1
         kind: Deployment
         name: prometheus
         namespace: monitor
     placement:
       clusterAffinity:
         clusterNames:
           - member1
           - member2
           - member3
   EOF
   cat <<EOF | kubectl apply -f -
   apiVersion: policy.karmada.io/v1alpha1
   kind: ClusterPropagationPolicy
   metadata:
     name: prometheusrbac-propagation
   spec:
     resourceSelectors:
       - apiVersion: rbac.authorization.k8s.io/v1
         kind: ClusterRole
         name: prometheus
       - apiVersion: rbac.authorization.k8s.io/v1
         kind: ClusterRoleBinding
         name: prometheus
     placement:
       clusterAffinity:
         clusterNames:
           - member1
           - member2
           - member3
   EOF
   ```

3. Use any node IP of the member cluster and the port number (default 30003) to enter the Prometheus monitoring page of the member cluster

## Reference

- https://github.com/prometheus/prometheus
- https://prometheus.io