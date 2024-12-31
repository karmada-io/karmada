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
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
        - /bin/karmada-webhook
        - --kubeconfig=/etc/karmada/kubeconfig
        - --bind-address=0.0.0.0
        - --metrics-bind-address=:8080
        - --default-not-ready-toleration-seconds=30
        - --default-unreachable-toleration-seconds=30
        - --secure-port=8443
        - --cert-dir=/var/serving-cert
        - --v=4
        ports:
        - containerPort: 8443
        - containerPort: 8080
          name: metrics
          protocol: TCP
        volumeMounts:
        - name: kubeconfig
          subPath: kubeconfig
          mountPath: /etc/karmada/kubeconfig
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
