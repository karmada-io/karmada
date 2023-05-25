package controlplane

const (
	// KubeControllerManagerDeployment is KubeControllerManage deployment manifest
	KubeControllerManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .DeploymentName }}
  namespace: {{ .Namespace }}
  labels:
    karmada-app: kube-controller-manager
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: kube-controller-manager
  template:
    metadata:
      labels:
        karmada-app: kube-controller-manager
    spec:
      automountServiceAccountToken: false
      priorityClassName: system-node-critical
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
              matchExpressions:
              - key: karmada-app
                operator: In
                values: ["kube-controller-manager"]
              topologyKey: kubernetes.io/hostname
      containers:
      - name: kube-controller-manager
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - kube-controller-manager
        - --allocate-node-cidrs=true
        - --kubeconfig=/etc/karmada/config
        - --authentication-kubeconfig=/etc/karmada/config
        - --authorization-kubeconfig=/etc/karmada/config
        - --bind-address=0.0.0.0
        - --client-ca-file=/etc/karmada/pki/ca.crt
        - --cluster-cidr=10.244.0.0/16
        - --cluster-name=karmada
        - --cluster-signing-cert-file=/etc/karmada/pki/ca.crt
        - --cluster-signing-key-file=/etc/karmada/pki/ca.key
        - --controllers=namespace,garbagecollector,serviceaccount-token,ttl-after-finished,bootstrapsigner,csrapproving,csrcleaner,csrsigning
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
        - name: kubeconfig
          mountPath: /etc/karmada/config
          subPath: config
      volumes:
        - name: karmada-certs
          secret:
            secretName: {{ .KarmadaCertsSecret }}
        - name: kubeconfig
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
    karmada-app: karmada-controller-manager
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-controller-manager
  template:
    metadata:
      labels:
        karmada-app: karmada-controller-manager
    spec:
      automountServiceAccountToken: false
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
      containers:
      - name: karmada-controller-manager
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - /bin/karmada-controller-manager
        - --kubeconfig=/etc/karmada/config
        - --bind-address=0.0.0.0
        - --cluster-status-update-frequency=10s
        - --secure-port=10357
        - --failover-eviction-timeout=30s
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
        volumeMounts:
        - name: kubeconfig
          subPath: config
          mountPath: /etc/karmada/config
      volumes:
      - name: kubeconfig
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
    karmada-app: karmada-scheduler
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-scheduler
  template:
    metadata:
      labels:
        karmada-app: karmada-scheduler
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
      - name: karmada-scheduler
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - /bin/karmada-scheduler
        - --kubeconfig=/etc/karmada/config
        - --bind-address=0.0.0.0
        - --secure-port=10351
        - --enable-scheduler-estimator=true
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
        volumeMounts:
        - name: kubeconfig
          subPath: config
          mountPath: /etc/karmada/config
      volumes:
        - name: kubeconfig
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
    karmada-app: karmada-descheduler
    app.kubernetes.io/managed-by: karmada-operator
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      karmada-app: karmada-descheduler
  template:
    metadata:
      labels:
        karmada-app: karmada-descheduler
    spec:
      automountServiceAccountToken: false
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
      - name: karmada-descheduler
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - /bin/karmada-descheduler
        - --kubeconfig=/etc/karmada/config
        - --bind-address=0.0.0.0
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
          subPath: config
          mountPath: /etc/karmada/config
      volumes:
        - name: kubeconfig
          secret:
            secretName: {{ .KubeconfigSecret }}
`
)
