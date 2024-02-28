# service-name-resolution-detector-example

The service name resolution detector is an example to detect coredns failure in member cluster.
It is deployed as DaemonSet in member cluster, each pod of which periodically looks up `kubernetes.default` service and
exports the status to node condition. The type of condition is named as `ServiceNameResolutionReady`.

There will be a leader who also collects all nodes conditions, figure out failure and sync the status to cluster conditions.

Here is a manifest you may need. And there are some notes:

- replace `<your-image-addr>` to your custom image address.
- replace `<your-cluster-name>` to the name of cluster where pods deployed.
- replace `<karmada-kubeconfig>` to contents of kubeconfig of karmada control plane.
- replace `<context-of-control-plane>` to the context of karmada kubeconfig, which refers to karmada control plane.

```yaml
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: service-name-resolution-detector
  namespace: kube-system
  labels:
    app: service-name-resolution-detector
spec:
  selector:
    matchLabels:
      app: service-name-resolution-detector
  template:
    metadata:
      labels:
        app: service-name-resolution-detector
    spec:
      containers:
        - image: <your-image-addr>
          name: service-name-resolution-detector
          command:
            - service-name-resolution-detector
              --karmada-kubeconfig=/tmp/config
              --karmada-context=<context-of-control-plane>
              --cluster-name=<your-cluster-name>
              --host-name=${HOST_NAME}
              --bind-address=${POD_ADDRESS}
              --healthz-port=8081
              --detectors=*
              --coredns-detect-period=5s
              --coredns-success-threshold=30s
              --coredns-failure-threshold=30s
              --coredns-stale-threshold=60s
          env:
            - name: POD_ADDRESS
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: status.podIP
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: metadata.name
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: metadata.namespace
            - name: HOST_NAME
              valueFrom:
                fieldRef:
                  apiVersion: v1
                  fieldPath: spec.nodeName
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
              scheme: HTTP
            initialDelaySeconds: 3
            timeoutSeconds: 3
            periodSeconds: 5
            successThreshold: 1
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8081
              scheme: HTTP
            initialDelaySeconds: 3
            timeoutSeconds: 3
            periodSeconds: 5
            successThreshold: 1
            failureThreshold: 3
          volumeMounts:
            - mountPath: /tmp
              name: karmada-config
      serviceAccountName: service-name-resolution-detector
      volumes:
        - configMap:
            name: karmada-kubeconfig
            items:
              - key: kubeconfig
                path: config
          name: karmada-config
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: service-name-resolution-detector
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: service-name-resolution-detector-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:service-name-resolution-detector
subjects:
  - kind: ServiceAccount
    name: service-name-resolution-detector
    namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:service-name-resolution-detector
rules:
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes/status
    verbs:
      - patch
      - update
  - apiGroups:
      - ""
      - events.k8s.io
    resources:
      - events
    verbs:
      - create
      - patch
      - update
  - apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    verbs:
      - get
      - list
      - watch
      - create
      - patch
      - update
      - delete
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: karmada-kubeconfig
  namespace: kube-system
data:
  kubeconfig: |+
    <karmada-kubeconfig>
```
