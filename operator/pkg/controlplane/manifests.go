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

package controlplane

const (
	// KubeControllerManagerDeployment is KubeControllerManager deployment manifest
	KubeControllerManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: kube-controller-manager
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-controller-manager
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kube-controller-manager
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app.kubernetes.io/name
                    operator: In
                    values:
                      - kube-controller-manager
                  - key: app.kubernetes.io/instance
                    operator: In
                    values:
                      - {{ .KarmadaInstanceName }}
              topologyKey: kubernetes.io/hostname
      containers:
        - name: kube-controller-manager
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          command:
            - kube-controller-manager
            - --allocate-node-cidrs=true
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --authentication-kubeconfig=/etc/karmada/config/karmada.config
            - --authorization-kubeconfig=/etc/karmada/config/karmada.config
            - --bind-address=0.0.0.0
            - --client-ca-file=/etc/karmada/pki/ca.crt
            - --cluster-cidr=10.244.0.0/16
            - --cluster-name=karmada
            - --cluster-signing-cert-file=/etc/karmada/pki/ca.crt
            - --cluster-signing-key-file=/etc/karmada/pki/ca.key
            - --controllers=namespace,garbagecollector,serviceaccount-token,ttl-after-finished,bootstrapsigner,csrcleaner,csrsigning,clusterrole-aggregation
            - --leader-elect=true
            - --node-cidr-mask-size=24
            - --root-ca-file=/etc/karmada/pki/ca.crt
            - --service-account-private-key-file=/etc/karmada/pki/karmada.key
            - --service-cluster-ip-range=10.96.0.0/12
            - --use-service-account-credentials=true
            - --v=4
          livenessProbe:
            failureThreshold: 8
            httpGet:
              path: /healthz
              port: 10257
              scheme: HTTPS
            initialDelaySeconds: 10
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 15
          volumeMounts:
            - name: karmada-certs
              mountPath: /etc/karmada/pki
              readOnly: true
            - name: karmada-config
              mountPath: /etc/karmada/config
      volumes:
        - name: karmada-certs
          secret:
            secretName: {{ .KarmadaCertsSecret }}
        - name: karmada-config
          secret:
            secretName: {{ .KubeconfigSecret }}
`

	// KamradaControllerManagerDeployment is karmada controllerManager Deployment manifest
	KamradaControllerManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: karmada-controller-manager
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: karmada-controller-manager
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: karmada-controller-manager
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-controller-manager
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-controller-manager
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --cluster-status-update-frequency=10s
            - --leader-elect-resource-namespace={{ .SystemNamespace }}
            - --metrics-bind-address=$(POD_IP):8080
            - --health-probe-bind-address=$(POD_IP):10357
            - --v=4
          livenessProbe:
            httpGet:
              path: /healthz
              port: 10357
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
            - name: karmada-config
              mountPath: /etc/karmada/config
      volumes:
        - name: karmada-config
          secret:
            secretName: {{ .KubeconfigSecret }}
`

	// KarmadaSchedulerDeployment is KarmadaScheduler Deployment manifest
	KarmadaSchedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: karmada-scheduler
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: karmada-scheduler
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: karmada-scheduler
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-scheduler
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-scheduler
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --metrics-bind-address=$(POD_IP):8080
            - --health-probe-bind-address=$(POD_IP):10351
            - --enable-scheduler-estimator=true
            - --leader-elect-resource-namespace={{ .SystemNamespace }}
            - --scheduler-estimator-ca-file=/etc/karmada/pki/ca.crt
            - --scheduler-estimator-cert-file=/etc/karmada/pki/karmada.crt
            - --scheduler-estimator-key-file=/etc/karmada/pki/karmada.key
            - --v=4
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
            - name: karmada-certs
              mountPath: /etc/karmada/pki
              readOnly: true
            - name: karmada-config
              mountPath: /etc/karmada/config
      volumes:
        - name: karmada-certs
          secret:
            secretName: {{ .KarmadaCertsSecret }}
        - name: karmada-config
          secret:
            secretName: {{ .KubeconfigSecret }}
`

	// KarmadaDeschedulerDeployment is KarmadaDescheduler Deployment manifest
	KarmadaDeschedulerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: karmada-descheduler
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: karmada-descheduler
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: karmada-descheduler
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-descheduler
          image: {{ .Image }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          command:
            - /bin/karmada-descheduler
            - --kubeconfig=/etc/karmada/config/karmada.config
            - --metrics-bind-address=$(POD_IP):8080
            - --health-probe-bind-address=$(POD_IP):10358
            - --leader-elect-resource-namespace={{ .SystemNamespace }}
            - --scheduler-estimator-ca-file=/etc/karmada/pki/ca.crt
            - --scheduler-estimator-cert-file=/etc/karmada/pki/karmada.crt
            - --scheduler-estimator-key-file=/etc/karmada/pki/karmada.key
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
          ports:
            - containerPort: 8080
              name: metrics
              protocol: TCP
          volumeMounts:
            - name: karmada-certs
              mountPath: /etc/karmada/pki
              readOnly: true
            - name: karmada-config
              mountPath: /etc/karmada/config
      volumes:
        - name: karmada-certs
          secret:
            secretName: {{ .KarmadaCertsSecret }}
        - name: karmada-config
          secret:
            secretName: {{ .KubeconfigSecret }}
`
)
