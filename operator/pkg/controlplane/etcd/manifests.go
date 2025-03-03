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

package etcd

const (
	// KarmadaEtcdStatefulSet is karmada etcd StatefulSet manifest
	KarmadaEtcdStatefulSet = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/name: etcd
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    app.kubernetes.io/managed-by: karmada-operator
  namespace: {{ .Namespace }}
  name: {{ .StatefulSetName }}
spec:
  replicas: {{ .Replicas }}
  serviceName: {{ .StatefulSetName }}
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app.kubernetes.io/name: etcd
      app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: etcd
        app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
    spec:
      automountServiceAccountToken: false
      containers:
      - name: etcd
        image: {{ .Image }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
        - /usr/local/bin/etcd
        - --name=$(KARMADA_ETCD_NAME)
        - --listen-client-urls=https://0.0.0.0:{{ .EtcdListenClientPort }}
        - --listen-peer-urls=http://0.0.0.0:{{ .EtcdListenPeerPort }}
        - --advertise-client-urls=https://{{ .EtcdClientService }}.{{ .Namespace }}.svc.cluster.local:{{ .EtcdListenClientPort }}
        - --initial-cluster={{ .InitialCluster }}
        - --initial-cluster-state=new
        - --client-cert-auth=true
        - --trusted-ca-file=/etc/karmada/pki/etcd/etcd-ca.crt
        - --cert-file=/etc/karmada/pki/etcd/etcd-server.crt
        - --key-file=/etc/karmada/pki/etcd/etcd-server.key
        - --data-dir=/var/lib/etcd
        - --snapshot-count=10000
        - --log-level=debug
        - --cipher-suites={{ .EtcdCipherSuites }}
        env:
        - name: KARMADA_ETCD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
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
          name: {{ .EtcdDataVolumeName }}
        - mountPath: /etc/karmada/pki/etcd
          name: etcd-cert
      volumes:
      - name: etcd-cert
        secret:
          secretName: {{ .CertsSecretName }}
`

	// KarmadaEtcdClientService is karmada etcd client service manifest
	KarmadaEtcdClientService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/name: etcd
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
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
    app.kubernetes.io/name: etcd
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  type: ClusterIP
`

	// KarmadaEtcdPeerService is karmada etcd peer Service manifest
	KarmadaEtcdPeerService = `
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/name: etcd
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
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
  selector:
    app.kubernetes.io/name: etcd
    app.kubernetes.io/instance: {{ .KarmadaInstanceName }}
  type: ClusterIP
`
)
