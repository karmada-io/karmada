/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
      priorityClassName: {{ .PriorityClassName }}
      containers:
        - name: karmada-metrics-adapter
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-metrics-adapter
            - --metrics-bind-address=$(POD_IP):8080
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --authentication-kubeconfig=/etc/karmada/config/karmada.config
            - --authorization-kubeconfig=/etc/karmada/config/karmada.config
            - --client-ca-file=/etc/karmada/pki/ca.crt
            - --tls-cert-file=/etc/karmada/pki/karmada.crt
            - --tls-private-key-file=/etc/karmada/pki/karmada.key
            - --audit-log-path=-
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0
            - --tls-min-version=VersionTLS13
            - --bind-address=$(POD_IP)
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
          volumeMounts:
            - name: karmada-config
              mountPath: /etc/karmada/config
            - name: k8s-certs
              mountPath: /etc/karmada/pki
              readOnly: true
      volumes:
        - name: karmada-config
          secret:
            secretName: karmada-metrics-adapter-config
        - name: k8s-certs
          secret:
            secretName: karmada-cert
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
	Namespace         string
	Replicas          *int32
	Image             string
	PriorityClassName string
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
