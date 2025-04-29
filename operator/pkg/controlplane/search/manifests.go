/*
Copyright 2023 The Karmada Authors.

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
	// KarmadaSearchDeployment is karmada search deployment manifest
	KarmadaSearchDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: karmada-search
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
    apiserver: "true"
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: karmada-search
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
      apiserver: "true"
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: karmada-search
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
        apiserver: "true"
    spec:
      automountServiceAccountToken: false
      containers:
        - name: karmada-search
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          volumeMounts:
            - name: k8s-certs
              mountPath: /etc/karmada/pki
              readOnly: true
            - name: karmada-config
              mountPath: /etc/karmada/config
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
            - --tls-cert-file=/etc/karmada/pki/karmada.crt
            - --tls-private-key-file=/etc/karmada/pki/karmada.key
            - --tls-min-version=VersionTLS13
            - --audit-log-path=-
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0
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
      volumes:
        - name: k8s-certs
          secret:
            secretName: {{ .KarmadaCertsSecret }}
        - name: karmada-config
          secret:
            secretName: {{ .KubeconfigSecret }}
`

	// KarmadaSearchService is karmada-search service manifest
	KarmadaSearchService = `
apiVersion: v1
kind: Service
metadata:
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: karmada-search
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
    apiserver: "true"
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: 443
  selector:
    app.kubernetes.io/name: karmada-search
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    apiserver: "true"
`
)
