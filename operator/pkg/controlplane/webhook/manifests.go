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
    app.kubernetes.io/name: karmada-webhook
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: karmada-webhook
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: karmada-webhook
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-webhook
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-webhook
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --bind-address=$(POD_IP)
            - --metrics-bind-address=$(POD_IP):8080
            - --health-probe-bind-address=$(POD_IP):8000
            - --secure-port=8443
            - --cert-dir=/var/serving-cert
            - --v=4
          ports:
            - containerPort: 8443
            - containerPort: 8080
              name: metrics
              protocol: TCP
          volumeMounts:
            - name: karmada-config
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
        - name: karmada-config
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
    app.kubernetes.io/name: karmada-webhook
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  selector:
    app.kubernetes.io/name: karmada-webhook
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  ports:
    - port: 443
      targetPort: 8443
`
)
