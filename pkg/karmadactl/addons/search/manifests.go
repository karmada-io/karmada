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
      priorityClassName: {{ .PriorityClassName }}
      automountServiceAccountToken: false
      containers:
        - name: karmada-search
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-search
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --authentication-kubeconfig=/etc/karmada/config/karmada.config
            - --authorization-kubeconfig=/etc/karmada/config/karmada.config
            - --etcd-servers={{ .ETCDSevers }}
            - --etcd-cafile=/etc/karmada/pki/etcd-ca.crt
            - --etcd-certfile=/etc/karmada/pki/etcd-client.crt
            - --etcd-keyfile=/etc/karmada/pki/etcd-client.key
            - --tls-cert-file=/etc/karmada/pki/karmada.crt
            - --tls-private-key-file=/etc/karmada/pki/karmada.key
            - --tls-min-version=VersionTLS13
            - --audit-log-path=-
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0{{- if .KeyPrefix }}
            - --etcd-prefix={{ .KeyPrefix }}{{- end }}
            - --bind-address=$(POD_IP)
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
          volumeMounts:
            - name: karmada-config
              mountPath: /etc/karmada/config
            - name: k8s-certs
              mountPath: /etc/karmada/pki
              readOnly: true
      volumes:
        - name: karmada-config
          secret:
            secretName: karmada-search-config
        - name: k8s-certs
          secret:
            secretName: karmada-cert
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
	Namespace         string
	Replicas          *int32
	Image             string
	ETCDSevers        string
	KeyPrefix         string
	PriorityClassName string
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
