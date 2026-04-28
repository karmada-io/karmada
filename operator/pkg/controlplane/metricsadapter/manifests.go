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
    app.kubernetes.io/name: karmada-metrics-adapter
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: karmada-metrics-adapter
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: karmada-metrics-adapter
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-metrics-adapter
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-metrics-adapter
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --metrics-bind-address=$(POD_IP):8080
            - --authentication-kubeconfig=/etc/karmada/config/karmada.config
            - --authorization-kubeconfig=/etc/karmada/config/karmada.config
            - --client-ca-file=/etc/karmada/pki/ca.crt
            - --tls-cert-file=/etc/karmada/pki/karmada.crt
            - --tls-private-key-file=/etc/karmada/pki/karmada.key
            - --tls-min-version=VersionTLS13
            - --audit-log-path=-
            - --audit-log-maxage=0
            - --audit-log-maxbackup=0
            - --bind-address=$(POD_IP)
          volumeMounts:
            - name: karmada-config
              mountPath: /etc/karmada/config
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
        - name: karmada-config
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
    app.kubernetes.io/name: karmada-metrics-adapter
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  ports:
    - port: 443
      protocol: TCP
      targetPort: 443
`
)
