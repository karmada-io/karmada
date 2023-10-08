package metricsadapter

const (
	// KarmadaMetricsAdapterDeployment is karmada-metrics-adapter deployment manifest
	KarmadaMetricsAdapterDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    karmada-app: karmada-metrics-adapter
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-metrics-adapter
  template:
    metadata:
      labels:
        karmada-app: karmada-metrics-adapter
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
      - name: karmada-metrics-adapter
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - /bin/karmada-metrics-adapter
        - --kubeconfig=/etc/karmada/kubeconfig
        - --authentication-kubeconfig=/etc/karmada/kubeconfig
        - --authorization-kubeconfig=/etc/karmada/kubeconfig
        - --client-ca-file=/etc/karmada/pki/ca.crt
        - --audit-log-path=-
        - --audit-log-maxage=0
        - --audit-log-maxbackup=0
        volumeMounts:
        - name: kubeconfig
          subPath: kubeconfig
          mountPath: /etc/karmada/kubeconfig
        - name: karmada-cert
          mountPath: /etc/karmada/pki
          readOnly: true
        readinessProbe:
          httpGet:
            path: /readyz
            port: 443
            scheme: HTTPS
          initialDelaySeconds: 1
          failureThreshold: 3
          periodSeconds: 3
          timeoutSeconds: 15
        livenessProbe:
          httpGet:
            path: /healthz
            port: 443
            scheme: HTTPS
          initialDelaySeconds: 10
          failureThreshold: 3
          periodSeconds: 10
          timeoutSeconds: 15
        resources:
          requests:
            cpu: 100m
      volumes:
      - name: kubeconfig
        secret:
          secretName: {{ .KubeconfigSecret }}
      - name: karmada-cert
        secret:
          secretName: {{ .KarmadaCertsSecret }}
`

	// KarmadaMetricsAdapterService is karmada-metrics-adapter service manifest
	KarmadaMetricsAdapterService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: karmada-operator
spec:
  selector:
    karmada-app: karmada-metrics-adapter
  ports:
  - port: 443
    protocol: TCP
    targetPort: 443
`
)
