/*
Copyright 2022 The Karmada Authors.

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

package estimator

const (
	karmadaEstimatorDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-scheduler-estimator-{{ .MemberClusterName}}
  namespace: {{ .Namespace }}
  labels:
    cluster: {{ .MemberClusterName}}
spec:
  selector:
    matchLabels:
      app: karmada-scheduler-estimator-{{ .MemberClusterName}}
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app: karmada-scheduler-estimator-{{ .MemberClusterName}}
        cluster: {{ .MemberClusterName}}
    spec:
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      automountServiceAccountToken: false
      priorityClassName: {{ .PriorityClassName }}
      containers:
        - name: karmada-scheduler-estimator
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-scheduler-estimator
            - --kubeconfig=/etc/{{ .MemberClusterName}}-kubeconfig
            - --cluster-name={{ .MemberClusterName}}
            - --grpc-auth-cert-file=/etc/karmada/pki/karmada.crt
            - --grpc-auth-key-file=/etc/karmada/pki/karmada.key
            - --grpc-client-ca-file=/etc/karmada/pki/ca.crt
            - --metrics-bind-address=$(POD_IP):8080
            - --health-probe-bind-address=$(POD_IP):10351
          livenessProbe:
            httpGet:
              path: /healthz
              port: 10351
              scheme: HTTP
            failureThreshold: 3
            initialDelaySeconds: 15
            periodSeconds: 15
            timeoutSeconds: 5
          ports:
            - containerPort: 8080
              name: metrics
              protocol: TCP
          volumeMounts:
            - name: k8s-certs
              mountPath: /etc/karmada/pki
              readOnly: true
            - name: member-kubeconfig
              subPath: {{ .MemberClusterName}}-kubeconfig
              mountPath: /etc/{{ .MemberClusterName}}-kubeconfig
      volumes:
        - name: k8s-certs
          secret:
            secretName: karmada-cert
        - name: member-kubeconfig
          secret:
            secretName: {{ .MemberClusterName}}-kubeconfig

`

	karmadaEstimatorService = `
apiVersion: v1
kind: Service
metadata:
  name: karmada-scheduler-estimator-{{ .MemberClusterName}}
  namespace: {{ .Namespace }}
  labels:
    cluster: {{ .MemberClusterName}}
spec:
  selector:
    app: karmada-scheduler-estimator-{{ .MemberClusterName}}
  ports:
    - protocol: TCP
      port: 10352
      targetPort: 10352
`
)

// DeploymentReplace is a struct to help to concrete
// the karmada-estimator deployment bytes with the deployment template
type DeploymentReplace struct {
	Namespace         string
	Replicas          *int32
	Image             string
	MemberClusterName string
	PriorityClassName string
}

// ServiceReplace is a struct to help to concrete
// the karmada-estimator Service bytes with the Service template
type ServiceReplace struct {
	Namespace         string
	MemberClusterName string
}
