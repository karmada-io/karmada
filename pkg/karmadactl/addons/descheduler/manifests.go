/*
Copyright 2020 The Karmada Authors.

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

package descheduler

const karmadaDeschedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-descheduler
  namespace: {{ .Namespace }}
  labels:
    app: karmada-descheduler
spec:
  selector:
    matchLabels:
      app: karmada-descheduler
  replicas: {{ .Replicas }}
  template:
    metadata:
      labels:
        app: karmada-descheduler
    spec:
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      automountServiceAccountToken: false
      containers:
        - name: karmada-descheduler
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          command:
            - /bin/karmada-descheduler
            - --kubeconfig=/etc/kubeconfig
            - --bind-address=0.0.0.0
            - --leader-elect-resource-namespace={{ .Namespace }}
            - --v=4
          livenessProbe:
            httpGet:
              path: /healthz
              port: 10358
              scheme: HTTP
            failureThreshold: 3
            initialDelaySeconds: 15
            periodSeconds: 15
            timeoutSeconds: 5
          volumeMounts:
            - name: kubeconfig
              subPath: kubeconfig
              mountPath: /etc/kubeconfig
      volumes:
        - name: kubeconfig
          secret:
            secretName: kubeconfig
`

// DeploymentReplace is a struct to help to concrete
// the karmada-descheduler deployment bytes with the deployment template
type DeploymentReplace struct {
	Namespace string
	Replicas  *int32
	Image     string
}
