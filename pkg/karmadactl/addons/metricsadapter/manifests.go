package metricsadapter

const (
	karmadaMetricsAdapterDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-metrics-adapter
  namespace: {{ .Namespace }}
  labels:
    app: karmada-metrics-adapter
    apiserver: "true"
spec:
  selector:
    matchLabels:
      app: karmada-metrics-adapter
      apiserver: "true"
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app: karmada-metrics-adapter
        apiserver: "true"
    spec:
      automountServiceAccountToken: false
      containers:
        - name: karmada-metrics-adapter
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: k8s-certs
              mountPath: /etc/karmada/pki
              readOnly: true
            - name: kubeconfig
              subPath: kubeconfig
              mountPath: /etc/kubeconfig
          command:
            - /bin/karmada-metrics-adapter
            - --kubeconfig=/etc/kubeconfig
            - --authentication-kubeconfig=/etc/kubeconfig
            - --authorization-kubeconfig=/etc/kubeconfig
            - --client-ca-file=/etc/karmada/pki/ca.crt
            - --audit-log-path=-
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0
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
        - name: k8s-certs
          secret:
            secretName: karmada-cert
        - name: kubeconfig
          secret:
            secretName: kubeconfig
`

	karmadaMetricsAdapterService = `
apiVersion: v1
kind: Service
metadata:
  name: karmada-metrics-adapter
  namespace: {{ .Namespace }}
  labels:
    app: karmada-metrics-adapter
    apiserver: "true"
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: 443
  selector:
    app: karmada-metrics-adapter
`

	karmadaMetricsAdapterAAAPIService = `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: {{ .Name }}
spec:
  service:
    name: karmada-metrics-adapter
    namespace:  {{ .Namespace }}
  group: {{ .Group }}
  version:  {{ .Version }}
  caBundle: {{ .CABundle }}
  groupPriorityMinimum: 100
  versionPriority: 200
`

	karmadaMetricsAdapterAAService = `
apiVersion: v1
kind: Service
metadata:
  name: karmada-metrics-adapter
  namespace: {{ .Namespace }}
spec:
  type: ExternalName
  externalName: karmada-metrics-adapter.{{ .Namespace }}.svc.{{ .HostClusterDomain }}
`
)

// DeploymentReplace is a struct to help to concrete
// the karmada-metrics-adapter deployment bytes with the deployment template
type DeploymentReplace struct {
	Namespace string
	Replicas  *int32
	Image     string
}

// ServiceReplace is a struct to help to concrete
// the karmada-metrics-adapter Service bytes with the Service template
type ServiceReplace struct {
	Namespace string
}

// AAApiServiceReplace is a struct to help to concrete
// the karmada-metrics-adapter ApiService bytes with the AAApiService template
type AAApiServiceReplace struct {
	Name      string
	Namespace string
	Group     string
	Version   string
	CABundle  string
}

// AAServiceReplace is a struct to help to concrete
// the karmada-metrics-adapter AA Service bytes with the AAService template
type AAServiceReplace struct {
	Namespace         string
	HostClusterDomain string
}
