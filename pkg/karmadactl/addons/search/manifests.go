package search

const (
	karmadaSearchDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-search
  namespace: {{ .Namespace }}
  labels:
    app: karmada-search
    apiserver: "true"
spec:
  selector:
    matchLabels:
      app: karmada-search
      apiserver: "true"
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app: karmada-search
        apiserver: "true"
    spec:
      automountServiceAccountToken: false
      containers:
        - name: karmada-search
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
            - /bin/karmada-search
            - --kubeconfig=/etc/kubeconfig
            - --authentication-kubeconfig=/etc/kubeconfig
            - --authorization-kubeconfig=/etc/kubeconfig
            - --etcd-servers={{ .ETCDSevers }}
            - --etcd-cafile=/etc/karmada/pki/etcd-ca.crt
            - --etcd-certfile=/etc/karmada/pki/etcd-client.crt
            - --etcd-keyfile=/etc/karmada/pki/etcd-client.key
            - --tls-cert-file=/etc/karmada/pki/karmada.crt
            - --tls-private-key-file=/etc/karmada/pki/karmada.key
            - --tls-min-version=VersionTLS13
            - --audit-log-path=-
            - --feature-gates=APIPriorityAndFairness=false
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0{{- if .KeyPrefix }}
            - --etcd-prefix={{ .KeyPrefix }}{{- end }}
          livenessProbe:
            httpGet:
              path: /livez
              port: 443
              scheme: HTTPS
            failureThreshold: 3
            initialDelaySeconds: 15
            periodSeconds: 15
            timeoutSeconds: 5
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

	karmadaSearchService = `
apiVersion: v1
kind: Service
metadata:
  name: karmada-search
  namespace: {{ .Namespace }}
  labels:
    app: karmada-search
    apiserver: "true"
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: 443
  selector:
    app: karmada-search
`

	karmadaSearchAAAPIService = `
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: {{ .Name }}
  labels:
    app: karmada-search
    apiserver: "true"
spec:
  caBundle: {{ .CABundle }}
  group: search.karmada.io
  groupPriorityMinimum: 2000
  service:
    name: karmada-search
    namespace: {{ .Namespace }}
  version: v1alpha1
  versionPriority: 10
`

	karmadaSearchAAService = `
apiVersion: v1
kind: Service
metadata:
  name: karmada-search
  namespace: {{ .Namespace }}
spec:
  type: ExternalName
  externalName: karmada-search.{{ .Namespace }}.svc.{{ .HostClusterDomain }}
`
)

// DeploymentReplace is a struct to help to concrete
// the karmada-search deployment bytes with the deployment template
type DeploymentReplace struct {
	Namespace  string
	Replicas   *int32
	Image      string
	ETCDSevers string
	KeyPrefix  string
}

// ServiceReplace is a struct to help to concrete
// the karmada-search Service bytes with the Service template
type ServiceReplace struct {
	Namespace string
}

// AAApiServiceReplace is a struct to help to concrete
// the karmada-search ApiService bytes with the AAApiService template
type AAApiServiceReplace struct {
	Name      string
	Namespace string
	CABundle  string
}

// AAServiceReplace is a struct to help to concrete
// the karmada-search AA Service bytes with the AAService template
type AAServiceReplace struct {
	Namespace         string
	HostClusterDomain string
}
