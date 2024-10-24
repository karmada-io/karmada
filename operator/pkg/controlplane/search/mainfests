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
    app.kubernetes.io/managed-by: karmada-operator
    karmada-app: karmada-search
    apiserver: "true"
spec:
  selector:
    matchLabels:
      karmada-app: karmada-search
      apiserver: "true"
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        karmada-app: karmada-search
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
            - name: kubeconfig
              subPath: kubeconfig
              mountPath: /etc/kubeconfig
          command:
            - /bin/karmada-search
            - --kubeconfig=/etc/kubeconfig
            - --authentication-kubeconfig=/etc/kubeconfig
            - --authorization-kubeconfig=/etc/kubeconfig
            - --tls-cert-file=/etc/karmada/pki/karmada.crt
            - --tls-private-key-file=/etc/karmada/pki/karmada.key
            - --tls-min-version=VersionTLS13
            - --audit-log-path=-
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0
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
        - name: kubeconfig
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
    app.kubernetes.io/managed-by: karmada-operator
    karmada-app: karmada-search
    apiserver: "true"
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: 443
  selector:
    karmada-app: karmada-search
`
)
