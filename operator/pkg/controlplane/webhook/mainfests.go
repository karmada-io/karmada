package webhook

const (
	// KarmadaWebhookDeployment is karmada webhook deployment manifest
	KarmadaWebhookDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    karmada-app: karmada-webhook
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-webhook
  template:
    metadata:
      labels:
        karmada-app: karmada-webhook
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
      - name: karmada-webhook
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - /bin/karmada-webhook
        - --kubeconfig=/etc/karmada/config
        - --bind-address=0.0.0.0
        - --default-not-ready-toleration-seconds=30
        - --default-unreachable-toleration-seconds=30
        - --secure-port=8443
        - --cert-dir=/var/serving-cert
        - --v=4
        ports:
        - containerPort: 8443
        volumeMounts:
        - name: kubeconfig
          subPath: config
          mountPath: /etc/karmada/config
        - name: cert
          mountPath: /var/serving-cert
          readOnly: true
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
      volumes:
      - name: kubeconfig
        secret:
          secretName: {{ .KubeconfigSecret }}
      - name: cert
        secret:
          secretName: {{ .WebhookCertsSecret }}
`

	// KarmadaWebhookService is karmada webhook service manifest
	KarmadaWebhookService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: karmada-operator
spec:
  selector:
    karmada-app: karmada-webhook
  ports:
  - port: 443
    targetPort: 8443
`
)
