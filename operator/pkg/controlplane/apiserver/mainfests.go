package apiserver

const (
	// KarmadaApiserverDeployment is karmada apiserver deployment manifest
	KarmadaApiserverDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    karmada-app: kube-apiserver
    app.kubernetes.io/managed-by: karmada-operator
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: kube-apiserver
  template:
    metadata:
      labels:
        karmada-app: kube-apiserver
    spec:
      automountServiceAccountToken: false
      containers:
      - name: kube-apiserver
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
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
        - --kubelet-client-certificate=/etc/karmada/pki/karmada.crt
        - --kubelet-client-key=/etc/karmada/pki/karmada.key
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --secure-port=5443
        - --service-account-issuer=https://kubernetes.default.svc.cluster.local
        - --service-account-key-file=/etc/karmada/pki/karmada.key
        - --service-account-signing-key-file=/etc/karmada/pki/karmada.key
        - --service-cluster-ip-range={{ .ServiceSubnet }}
        - --proxy-client-cert-file=/etc/karmada/pki/front-proxy-client.crt
        - --proxy-client-key-file=/etc/karmada/pki/front-proxy-client.key
        - --requestheader-allowed-names=front-proxy-client
        - --requestheader-client-ca-file=/etc/karmada/pki/front-proxy-ca.crt
        - --requestheader-extra-headers-prefix=X-Remote-Extra-
        - --requestheader-group-headers=X-Remote-Group
        - --requestheader-username-headers=X-Remote-User
        - --tls-cert-file=/etc/karmada/pki/apiserver.crt
        - --tls-private-key-file=/etc/karmada/pki/apiserver.key
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
          name: apiserver-cert
          readOnly: true
        - mountPath: /etc/etcd/pki
          name: etcd-cert
          readOnly: true
      priorityClassName: system-node-critical
      volumes:
      - name: apiserver-cert
        secret:
          secretName: {{ .KarmadaCertsSecret }}
      - name: etcd-cert
        secret:
          secretName: {{ .EtcdCertsSecret }}
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
    karmada-app: kube-apiserver
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
        imagePullPolicy: IfNotPresent
        command:
        - /bin/karmada-aggregated-apiserver
        - --kubeconfig=/etc/karmada/config
        - --authentication-kubeconfig=/etc/karmada/config
        - --authorization-kubeconfig=/etc/karmada/config
        - --etcd-cafile=/etc/etcd/pki/etcd-ca.crt
        - --etcd-certfile=/etc/etcd/pki/etcd-client.crt
        - --etcd-keyfile=/etc/etcd/pki/etcd-client.key
        - --etcd-servers=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --tls-cert-file=/etc/karmada/pki/karmada.crt
        - --tls-private-key-file=/etc/karmada/pki/karmada.key
        - --audit-log-path=-
        - --feature-gates=APIPriorityAndFairness=false
        - --audit-log-maxage=0
        - --audit-log-maxbackup=0
        volumeMounts:
        - mountPath: /etc/karmada/config
          name: kubeconfig
          subPath: config
        - mountPath: /etc/etcd/pki
          name: etcd-cert
          readOnly: true
        - mountPath: /etc/karmada/pki
          name: apiserver-cert
          readOnly: true
      volumes:
      - name: kubeconfig
        secret:
          secretName: {{ .KubeconfigSecret }}
      - name: apiserver-cert
        secret:
          secretName: {{ .KarmadaCertsSecret }}
      - name: etcd-cert
        secret:
          secretName: {{ .EtcdCertsSecret }}
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
