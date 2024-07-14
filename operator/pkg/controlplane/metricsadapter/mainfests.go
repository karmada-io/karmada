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
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
        - /bin/karmada-metrics-adapter
        - --kubeconfig=/etc/karmada/kubeconfig
        - --authentication-kubeconfig=/etc/karmada/kubeconfig
        - --authorization-kubeconfig=/etc/karmada/kubeconfig
        - --client-ca-file=/etc/karmada/pki/ca.crt
        - --tls-cert-file=/etc/karmada/pki/karmada.crt
        - --tls-private-key-file=/etc/karmada/pki/karmada.key
        - --tls-min-version=VersionTLS13
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
