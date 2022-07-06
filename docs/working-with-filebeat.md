# Use Filebeat to collect logs of Karmada member clusters

[Filebeat](https://github.com/elastic/beats/tree/master/filebeat) is a lightweight shipper for forwarding and centralizing log data. Installed as an agent on your servers, Filebeat monitors the log files or locations that you specify, collects log events, and forwards them either to [Elasticsearch](https://www.elastic.co/products/elasticsearch) or [kafka](https://github.com/apache/kafka) for indexing. 

This document demonstrates how to use the `Filebeat` to collect logs of Karmada member clusters. 

## Start up Karmada clusters

You just need to clone Karmada repo, and run the following script in Karmada directory. 

```bash
hack/local-up-karmada.sh
```

## Start Filebeat

1. Create resource objects of Filebeat, the content is as follows. You can specify a list of inputs in the `filebeat.inputs` section of the `filebeat.yml`. Inputs specify how Filebeat locates and processes input data, also you can configure Filebeat to write to a specific output by setting options in the `Outputs` section of the `filebeat.yml` config file. The example will collect the log information of each container and write the collected logs to a file. More detailed information about the input and output configuration, please refer to: https://github.com/elastic/beats/tree/master/filebeat/docs

   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: logging
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: filebeat
     namespace: logging
     labels:
       k8s-app: filebeat
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: filebeat
   rules:
   - apiGroups: [""] # "" indicates the core API group
     resources:
     - namespaces
     - pods
     verbs:
     - get
     - watch
     - list
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: filebeat
   subjects:
   - kind: ServiceAccount
     name: filebeat
     namespace: kube-system
   roleRef:
     kind: ClusterRole
     name: filebeat
     apiGroup: rbac.authorization.k8s.io
   ---
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: filebeat-config
     namespace: logging
     labels:
       k8s-app: filebeat
       kubernetes.io/cluster-service: "true"
   data:
     filebeat.yml: |-
       filebeat.inputs:
       - type: container
         paths:
           - /var/log/containers/*.log
         processors:
           - add_kubernetes_metadata:
               host: ${NODE_NAME}
               matchers:
               - logs_path:
                   logs_path: "/var/log/containers/"
       # To enable hints based autodiscover, remove `filebeat.inputs` configuration and uncomment this:
       #filebeat.autodiscover:
       #  providers:
       #    - type: kubernetes
       #      node: ${NODE_NAME}
       #      hints.enabled: true
       #      hints.default_config:
       #        type: container
       #        paths:
       #          - /var/log/containers/*${data.kubernetes.container.id}.log
   
       processors:
         - add_cloud_metadata:
         - add_host_metadata:
   
       #output.elasticsearch:
       #  hosts: ['${ELASTICSEARCH_HOST:elasticsearch}:${ELASTICSEARCH_PORT:9200}']
       #  username: ${ELASTICSEARCH_USERNAME}
       #  password: ${ELASTICSEARCH_PASSWORD}
       output.file:
           path: "/tmp/filebeat"
           filename: filebeat
   ---
   apiVersion: apps/v1
   kind: DaemonSet
   metadata:
     name: filebeat
     namespace: logging
     labels:
       k8s-app: filebeat
   spec:
     selector:
       matchLabels:
         k8s-app: filebeat
     template:
       metadata:
         labels:
           k8s-app: filebeat
       spec:
         serviceAccountName: filebeat
         terminationGracePeriodSeconds: 30
         tolerations:
         - effect: NoSchedule
           key: node-role.kubernetes.io/master
         containers:
         - name: filebeat
           image: docker.elastic.co/beats/filebeat:8.0.0-beta1-amd64
           imagePullPolicy: IfNotPresent
           args: [  "-c", "/usr/share/filebeat/filebeat.yml",  "-e",]
           env:
           - name: NODE_NAME
             valueFrom:
               fieldRef:
                 fieldPath: spec.nodeName
           securityContext:
             runAsUser: 0
           resources:
             limits:
               memory: 200Mi
             requests:
               cpu: 100m
               memory: 100Mi
           volumeMounts:
           - name: config
             mountPath: /usr/share/filebeat/filebeat.yml
             readOnly: true
             subPath: filebeat.yml
           - name: inputs
             mountPath: /usr/share/filebeat/inputs.d
             readOnly: true
           - name: data
             mountPath: /usr/share/filebeat/data
           - name: varlibdockercontainers
             mountPath: /var/lib/docker/containers
             readOnly: true
           - name: varlog
             mountPath: /var/log
             readOnly: true
         volumes:
         - name: config
           configMap:
             defaultMode: 0600
             name: filebeat-config
         - name: varlibdockercontainers
           hostPath:
             path: /var/lib/docker/containers
         - name: varlog
           hostPath:
             path: /var/log
         - name: inputs
           configMap:
             defaultMode: 0600
             name: filebeat-config
         # data folder stores a registry of read status for all files, so we don't send everything again on a Filebeat pod restart
         - name: data
           hostPath:
             path: /var/lib/filebeat-data
             type: DirectoryOrCreate
   ```

2. Run the below command to execute Karmada PropagationPolicy and ClusterPropagationPolicy. 

   ```
   cat <<EOF | kubectl apply -f -
   apiVersion: policy.karmada.io/v1alpha1
   kind: PropagationPolicy
   metadata:
     name: filebeat-propagation
     namespace: logging
   spec:
     resourceSelectors:
       - apiVersion: v1
         kind: Namespace
         name: logging
       - apiVersion: v1
         kind: ServiceAccount
         name: filebeat
         namespace: logging
       - apiVersion: v1
         kind: ConfigMap
         name: filebeat-config
         namespace: logging
       - apiVersion: apps/v1
         kind: DaemonSet
         name: filebeat
         namespace: logging
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
     name: filebeatsrbac-propagation
   spec:
     resourceSelectors:
       - apiVersion: rbac.authorization.k8s.io/v1
         kind: ClusterRole
         name: filebeat
       - apiVersion: rbac.authorization.k8s.io/v1
         kind: ClusterRoleBinding
         name: filebeat
     placement:
       clusterAffinity:
         clusterNames:
         - member1
         - member2
         - member3
   EOF
   ```

3. Obtain the collected logs according to the `output` configuration of the `filebeat.yml`.

## Reference

- https://github.com/elastic/beats/tree/master/filebeat
- https://github.com/elastic/beats/tree/master/filebeat/docs
