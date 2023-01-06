package etcd

const (
	// KarmadaEtcdStatefulSet is karmada etcd StatefulSet manifest
	KarmadaEtcdStatefulSet = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    karmada-app: etcd
    app.kubernetes.io/managed-by: karmada-operator
  namespace: {{ .Namespace }}
  name: {{ .StatefulSetName }}
spec:
  replicas: {{ .Replicas }}
  serviceName: {{ .StatefulSetName }}
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      karmada-app: etcd
  template:
    metadata:
      labels:
        karmada-app: etcd
    tolerations:
    - operator: Exists
    spec:
      automountServiceAccountToken: false
      containers:
      - name: etcd
        image: {{ .Image }}
        imagePullPolicy: IfNotPresent
        command:
        - /usr/local/bin/etcd
        - --name={{ .StatefulSetName }}0
        - --listen-client-urls= https://0.0.0.0:{{ .EtcdListenClientPort }}
        - --listen-peer-urls=http://0.0.0.0:{{ .EtcdListenPeerPort }}
        - --advertise-client-urls=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --initial-cluster={{ .StatefulSetName }}0=http://{{ .StatefulSetName }}-0.{{ .EtcdPeerServiceName }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenPeerPort }}
        - --initial-cluster-state=new
        - --client-cert-auth=true
        - --trusted-ca-file=/etc/karmada/pki/etcd/etcd-ca.crt
        - --cert-file=/etc/karmada/pki/etcd/etcd-server.crt
        - --key-file=/etc/karmada/pki/etcd/etcd-server.key
        - --data-dir=/var/lib/etcd
        - --snapshot-count=10000
        - --log-level=debug
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -ec
            - etcdctl get /registry --prefix --keys-only --endpoints https://127.0.0.1:{{ .EtcdListenClientPort }} --cacert=/etc/karmada/pki/etcd/etcd-ca.crt --cert=/etc/karmada/pki/etcd/etcd-server.crt --key=/etc/karmada/pki/etcd/etcd-server.key
          failureThreshold: 3
          initialDelaySeconds: 600
          periodSeconds: 60
          successThreshold: 1
          timeoutSeconds: 10
        ports:
        - containerPort: {{ .EtcdListenClientPort }}
          name: client
          protocol: TCP
        - containerPort: {{ .EtcdListenPeerPort }}
          name: server
          protocol: TCP
        volumeMounts:
        - mountPath: /var/lib/etcd
          name: etcd-data
        - mountPath: /etc/karmada/pki/etcd
          name: etcd-cert
      volumes:
      - name: etcd-cert
        secret:
          secretName: {{ .CertsSecretName }}
      - name: etcd-data
        emptyDir: {}
`

	// KarmadaEtcdClientService is karmada etcd client service manifest
	KarmadaEtcdClientService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    karmada-app: etcd
    app.kubernetes.io/managed-by: karmada-operator
  name: {{ .ServiceName }}
  namespace: {{ .Namespace }}
spec:
  ports:
  - name: client
    port: {{ .EtcdListenClientPort }}
    protocol: TCP
    targetPort: {{ .EtcdListenClientPort }}
  selector:
    karmada-app: etcd
  type: ClusterIP
 `

	// KarmadaEtcdPeerService is karmada etcd peer Service manifest
	KarmadaEtcdPeerService = `
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     karmada-app: etcd
     app.kubernetes.io/managed-by: karmada-operator
   name: {{ .ServiceName }}
   namespace: {{ .Namespace }}
 spec:
   clusterIP: None
   ports:
   - name: client
     port: {{ .EtcdListenClientPort }}
     protocol: TCP
     targetPort: {{ .EtcdListenClientPort }}
   - name: server
     port: {{ .EtcdListenPeerPort }}
     protocol: TCP
     targetPort: {{ .EtcdListenPeerPort }}
   type: ClusterIP
  `
)
