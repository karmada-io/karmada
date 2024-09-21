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

package apiserver

const (
	// KarmadaApiserverDeployment is karmada apiserver deployment manifest
	KarmadaApiserverDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    karmada-app: karmada-apiserver
    app.kubernetes.io/managed-by: karmada-operator
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-apiserver
  template:
    metadata:
      labels:
        karmada-app: karmada-apiserver
    spec:
      automountServiceAccountToken: false
      containers:
      - name: kube-apiserver
        image: {{ .Image }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
        - kube-apiserver
        - --allow-privileged=true
        - --authorization-mode=Node,RBAC
        - --client-ca-file=/etc/karmada/pki/ca.crt
        - --disable-admission-plugins=StorageObjectInUseProtection,ServiceAccount
        - --enable-admission-plugins=NodeRestriction
        - --enable-bootstrap-token-auth=true
        - --etcd-cafile=/etc/etcd/pki/etcd-ca.crt
        - --etcd-certfile=/etc/etcd/pki/etcd-client.crt
        - --etcd-keyfile=/etc/etcd/pki/etcd-client.key
        - --etcd-servers=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --bind-address=0.0.0.0
        - --secure-port=5443
        - --service-account-issuer=https://kubernetes.default.svc.cluster.local
        - --service-account-key-file=/etc/karmada/pki/karmada-client.key
        - --service-account-signing-key-file=/etc/karmada/pki/karmada-client.key
        - --service-cluster-ip-range={{ .ServiceSubnet }}
        - --proxy-client-cert-file=/etc/karmada/pki/front-proxy-client.crt
        - --proxy-client-key-file=/etc/karmada/pki/front-proxy-client.key
        - --requestheader-allowed-names=front-proxy-client
        - --requestheader-client-ca-file=/etc/karmada/pki/front-proxy-ca.crt
        - --requestheader-extra-headers-prefix=X-Remote-Extra-
        - --requestheader-group-headers=X-Remote-Group
        - --requestheader-username-headers=X-Remote-User
        - --tls-cert-file=/etc/karmada/pki/karmada-server.crt
        - --tls-private-key-file=/etc/karmada/pki/karmada-server.key
        - --tls-min-version=VersionTLS13
        - --max-requests-inflight=1500
        - --max-mutating-requests-inflight=500
        - --v=4

        livenessProbe:
          failureThreshold: 8
          httpGet:
            path: /livez
            port: 5443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 15
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /readyz
            port: 5443
            scheme: HTTPS
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 15
        affinity:
          podAntiAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
              matchExpressions:
              - key: karmada-app
                operator: In
                values:
                - karmada-apiserver
              topologyKey: kubernetes.io/hostname
        ports:
        - containerPort: 5443
          name: http
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/karmada/pki
          name: karmada-certs
          readOnly: true
        - mountPath: /etc/etcd/pki
          name: karmada-etcd-cert
          readOnly: true
      priorityClassName: system-node-critical
      volumes:
      - name: karmada-certs
        secret:
          secretName: {{ .KarmadaCertsSecret }}
      - name: karmada-etcd-cert
        secret:
          secretName: {{ .KarmadaEtcdCertSecret }}
`

	// KarmadaApiserverService is karmada apiserver service manifest
	KarmadaApiserverService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    karmada-app: karmada-apiserver
    app.kubernetes.io/managed-by: karmada-operator
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  ports:
  - name: client
    port: 5443
    protocol: TCP
    targetPort: 5443
  selector:
    karmada-app: karmada-apiserver
  type: {{ .ServiceType }}
`

	// KarmadaAggregatedAPIServerDeployment is karmada aggreagated apiserver deployment manifest
	KarmadaAggregatedAPIServerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    karmada-app: karmada-aggregated-apiserver
    app.kubernetes.io/managed-by: karmada-operator
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-aggregated-apiserver
  template:
    metadata:
      labels:
        karmada-app: karmada-aggregated-apiserver
    spec:
      automountServiceAccountToken: false
      containers:
      - name: karmada-aggregated-apiserver
        image: {{ .Image }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
        - /bin/karmada-aggregated-apiserver
        - --kubeconfig=/etc/kubeconfig
        - --authentication-kubeconfig=/etc/kubeconfig
        - --authorization-kubeconfig=/etc/kubeconfig
        - --etcd-cafile=/etc/etcd/pki/etcd-ca.crt
        - --etcd-certfile=/etc/etcd/pki/etcd-client.crt
        - --etcd-keyfile=/etc/etcd/pki/etcd-client.key
        - --etcd-servers=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --tls-cert-file=/etc/karmada/pki/karmada-server.crt
        - --tls-private-key-file=/etc/karmada/pki/karmada-server.key
        - --tls-min-version=VersionTLS13
        - --audit-log-path=-
        - --audit-log-maxage=0
        - --audit-log-maxbackup=0
        volumeMounts:
        - mountPath: /etc/kubeconfig
          name: karmada-kubeconfig
          subPath: kubeconfig
        - mountPath: /etc/etcd/pki
          name: karmada-etcd-cert
          readOnly: true
        - mountPath: /etc/karmada/pki
          name: karmada-certs
          readOnly: true
      volumes:
      - name: karmada-kubeconfig
        secret:
          secretName: {{ .KarmadaKubeconfigSecret }}
      - name: karmada-certs
        secret:
          secretName: {{ .KarmadaCertsSecret }}
      - name: karmada-etcd-cert
        secret:
          secretName: {{ .KarmadaEtcdCertSecret }}
`
	// KarmadaAggregatedAPIServerService is karmada aggregated APIServer Service manifest
	KarmadaAggregatedAPIServerService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    karmada-app: karmada-aggregated-apiserver
    app.kubernetes.io/managed-by: karmada-operator
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 443
  selector:
    karmada-app: karmada-aggregated-apiserver
  type: ClusterIP
`
)
