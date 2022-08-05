Step-by-step installation of binary high-availability `karmada` cluster.

# Installing Karmada cluster

## Prerequisites

#### server

3 servers required. E.g.

```shell
+---------------+-----------------+-----------------+
|   HostName    |      Host IP    |    Public IP    |
+---------------+-----------------+-----------------+
|  karmada-01   |  172.31.209.245 |  47.242.88.82   |
+---------------+-----------------+-----------------+
|  karmada-02   |  172.31.209.246 |                 |
+---------------+-----------------+-----------------+
|  karmada-03   |  172.31.209.247 |                 |
+---------------+-----------------+-----------------+
```

> Public IP is not required. It is used to download some `karmada` dependent components from the public network and connect to `karmada` ApiServer through the public network

#### hosts parsing

Execute operations at `karmada-01` `karmada-02` `karmada-03`.

```bash
vi /etc/hosts
172.31.209.245	karmada-01
172.31.209.246	karmada-02
172.31.209.247	karmada-03
```

#### environment

`karmada-01`  requires the following environment.

**Golang**:  Compile the karmada binary
**GCC**: Compile nginx (ignore if using cloud load balancing)







## Compile and download binaries

Execute operations at `karmada-01`.

#### kubernetes binaries

Download the `kubernetes` binary package.

```bash
wget https://dl.k8s.io/v1.23.3/kubernetes-server-linux-amd64.tar.gz
tar -zxvf kubernetes-server-linux-amd64.tar.gz
cd /root/kubernetes/server/bin
mv  kube-apiserver kube-controller-manager kubectl /usr/local/sbin/
```

#### etcd binaries

Download the `etcd` binary package.

```bash
wget https://github.com/etcd-io/etcd/releases/download/v3.5.1/etcd-v3.5.1-linux-amd64.tar.gz
tar -zxvf etcd-v3.5.1-linux-amd64.tar.gz
cd etcd-v3.5.1-linux-amd64/
cp etcdctl etcd /usr/local/sbin/
```

#### karmada binaries

Compile the `karmada` binary from source.

```bash
git clone https://github.com/karmada-io/karmada
cd karmada
make karmada-aggregated-apiserver
make karmada-controller-manager
make karmada-scheduler
make karmada-webhook
mv karmada-aggregated-apiserver karmada-controller-manager karmada-scheduler karmada-webhook /usr/local/sbin/
```

#### nginx binaries

Compile the `nginx` binary from source.

```bash
wget http://nginx.org/download/nginx-1.21.6.tar.gz
tar -zxvf nginx-1.21.6.tar.gz
cd nginx-1.21.6
./configure --with-stream --without-http --prefix=/usr/local/karmada-nginx --without-http_uwsgi_module --without-http_scgi_module --without-http_fastcgi_module
make && make install
mv /usr/local/karmada-nginx/sbin/nginx /usr/local/karmada-nginx/sbin/karmada-nginx
```

#### Distribute binaries

Upload the binary file to the `karmada-02`  `karmada-03 ` server

```bash
scp /usr/local/sbin/* karmada-02:/usr/local/sbin/
scp /usr/local/sbin/* karmada-03:/usr/local/sbin/
```



## Generate certificate

Generated using the `openssl` command. Note yes `DNS` and `IP` when generating the certificate.

Execute operations at `karmada-01`.

#### create a temporary directory for certificates

```bash
mkdir certs
cd certs
```

#### Create root certificate

valid for 10 years

```bash
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=karmada" -days 3650 -out ca.crt
```

#### Create etcd certificate

create `etcd server ` certificate

```bash
openssl genrsa -out etcd-server.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=karmada-etcd"  -key etcd-server.key -out etcd-server.csr
openssl x509 -req -days 3650  \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=serverAuth,clientAuth\nauthorityKeyIdentifier=keyid,issuer\nsubjectAltName=IP:172.31.209.245,IP:172.31.209.246,IP:172.31.209.247,IP:127.0.0.1,DNS:localhost") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in etcd-server.csr -out etcd-server.crt
```

create `etcd peer ` certificate

```bash
openssl genrsa -out etcd-peer.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=karmada-etcd-peer"  -key etcd-peer.key -out etcd-peer.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=serverAuth,clientAuth\nauthorityKeyIdentifier=keyid,issuer\nsubjectAltName=IP:172.31.209.245,IP:172.31.209.246,IP:172.31.209.247,IP:127.0.0.1,DNS:localhost") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in etcd-peer.csr -out etcd-peer.crt
```

create `etcd client ` certificate

```bash
openssl genrsa -out karmada-etcd-client.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=karmada-etcd-client"  -key karmada-etcd-client.key -out karmada-etcd-client.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=clientAuth\nauthorityKeyIdentifier=keyid,issuer") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in karmada-etcd-client.csr -out karmada-etcd-client.crt
```

#### Create karmada certificate

create `karmada-apiserver ` certificate.

>Notice:
>
>If you need to access the `karmada apiserver` through the public `IP/DNS` or external `IP/DNS`, the certificate needs to be added to the `IP/DNS`.

```bash
openssl genrsa -out karmada-apiserver.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=karmada" -key karmada-apiserver.key -out karmada-apiserver.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=serverAuth,clientAuth\nauthorityKeyIdentifier=keyid,issuer\nsubjectAltName=DNS:kubernetes,DNS:kubernetes.default,DNS:kubernetes.default.svc,DNS:kubernetes.default.svc.cluster,DNS:kubernetes.default.svc.cluster.local,IP:172.31.209.245,IP:172.31.209.246,IP:172.31.209.247,IP:10.254.0.1,IP:47.242.88.82") \
  -sha256  -CA ca.crt -CAkey ca.key -set_serial 01  -in karmada-apiserver.csr -out karmada-apiserver.crt
```

create `karmada admin  ` certificate.

```bash
openssl genrsa -out admin.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=system:masters/OU=System/CN=admin"  -key admin.key -out admin.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=clientAuth\nauthorityKeyIdentifier=keyid,issuer") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in admin.csr   -out admin.crt
```

create `kube-controller-manager ` certificate.

```bash
openssl genrsa -out kube-controller-manager.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=system:kube-controller-manager"  -key kube-controller-manager.key -out kube-controller-manager.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=clientAuth\nauthorityKeyIdentifier=keyid,issuer") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in kube-controller-manager.csr -out kube-controller-manager.crt
```

create `karmada components` certificate.

```bash
openssl genrsa -out karmada.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=system:karmada"  -key karmada.key -out karmada.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=serverAuth,clientAuth\nauthorityKeyIdentifier=keyid,issuer\nsubjectAltName=DNS:karmada-01,DNS:karmada-02,DNS:karmada-03,DNS:localhost,IP:172.0.0.1,IP:172.31.209.245,IP:172.31.209.246,IP:172.31.209.247,IP:10.254.0.1,IP:47.242.88.82") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in karmada.csr -out karmada.crt
```

create `front-proxy-client` certificate.

```bash
openssl genrsa -out front-proxy-client.key 2048
openssl req -new -nodes -sha256 -subj "/C=CN/ST=Guangdong/L=Guangzhou/O=karmada/OU=System/CN=front-proxy-client"  -key front-proxy-client.key -out front-proxy-client.csr
openssl x509 -req -days 3650 \
  -extfile <(printf "keyUsage=critical,Digital Signature, Key Encipherment\nextendedKeyUsage=serverAuth,clientAuth\nauthorityKeyIdentifier=keyid,issuer") \
  -sha256 -CA ca.crt -CAkey ca.key -set_serial 01 -in front-proxy-client.csr -out front-proxy-client.crt
```

create `karmada-apiserver`  SA key

```bash
openssl genrsa -out sa.key 2048
openssl rsa -in sa.key -pubout -out sa.pub
```

#### Check the certificate

You can view the configuration of the certificate, take `etcd-server `as an example.

```bash
openssl x509  -noout -text  -in  etcd-server.crt
```

#### Create the karmada configuration directory

copy the certificate to the `/etc/karmada/pki` directory.

```bash
mkdir -p /etc/karmada/pki
cp karmada.key tls.key
cp karmada.crt tls.crt
cp *.key *.crt sa.pub /etc/karmada/pki
```



## Create the karmada kubeconfig files and etcd encrypted file

Execute operations at `karmada-01`.

Define the karmada apiserver address. `172.31.209.245:5443` is the address of the `nginx` proxy `karmada-apiserver` ,we'll set it up later.

```bash
export KARMADA_APISERVER="https://172.31.209.245:5443"
cd /etc/karmada/
```

#### Create kubectl kubeconfig file

which is kept at $HOME/.kube/config by default

```bas
kubectl config set-cluster karmada \
  --certificate-authority=/etc/karmada/pki/ca.crt \
  --embed-certs=true \
  --server=${KARMADA_APISERVER}

kubectl config set-credentials admin \
  --client-certificate=/etc/karmada/pki/admin.crt \
  --embed-certs=true \
  --client-key=/etc/karmada/pki/admin.key

kubectl config set-context karmada \
  --cluster=karmada \
  --user=admin

kubectl config use-context karmada
```

#### Create kube-controller-manager kubeconfig file

```bash
kubectl config set-cluster karmada \
  --certificate-authority=/etc/karmada/pki/ca.crt \
  --embed-certs=true \
  --server=${KARMADA_APISERVER} \
  --kubeconfig=kube-controller-manager.kubeconfig

kubectl config set-credentials system:kube-controller-manager \
  --client-certificate=/etc/karmada/pki/kube-controller-manager.crt \
  --client-key=/etc/karmada/pki/kube-controller-manager.key \
  --embed-certs=true \
  --kubeconfig=kube-controller-manager.kubeconfig

kubectl config set-context system:kube-controller-manager \
  --cluster=karmada \
  --user=system:kube-controller-manager \
  --kubeconfig=kube-controller-manager.kubeconfig
  
kubectl config use-context system:kube-controller-manager --kubeconfig=kube-controller-manager.kubeconfig
```

#### Create karmada kubeconfig file

The components of karmada connect to the karmada apiserver through this file.

```bash
kubectl config set-cluster karmada \
  --certificate-authority=/etc/karmada/pki/ca.crt \
  --embed-certs=true \
  --server=${KARMADA_APISERVER} \
  --kubeconfig=karmada.kubeconfig

kubectl config set-credentials system:karmada \
  --client-certificate=/etc/karmada/pki/karmada.crt \
  --client-key=/etc/karmada/pki/karmada.key \
  --embed-certs=true \
  --kubeconfig=karmada.kubeconfig

kubectl config set-context system:karmada\
  --cluster=karmada \
  --user=system:karmada \
  --kubeconfig=karmada.kubeconfig

kubectl config use-context system:karmada --kubeconfig=karmada.kubeconfig
```

#### Create etcd encrypted file

```bash
export ENCRYPTION_KEY=$(head -c 32 /dev/urandom | base64)
cat > /etc/karmada/encryption-config.yaml <<EOF
kind: EncryptionConfig
apiVersion: v1
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: ${ENCRYPTION_KEY}
      - identity: {}
EOF
```

#### Distribution profile

package the `karmada` configuration file and copy it to other nodes.

```bash
cd /etc
tar -cvf karmada.tar karmada
scp karmada.tar  karmada-02:/etc/
scp karmada.tar  karmada-03:/etc/
```

`karmada-02`  `karmada-03 `  need to decompress

```bash
cd /etc
tar -xvf karmada.tar
```



## Install etcd cluster

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

#### Create systemd file

```bash
vi /usr/lib/systemd/system/etcd.service
[Unit]
Description=Etcd Server
After=network.target
After=network-online.target
Wants=network-online.target
Documentation=https://github.com/coreos

[Service]
Type=notify
WorkingDirectory=/var/lib/etcd/
ExecStart=/usr/local/sbin/etcd \
  --name karmada-01 \
  --client-cert-auth=true \
  --cert-file=/etc/karmada/pki/etcd-server.crt \
  --key-file=/etc/karmada/pki/etcd-server.key \
  --peer-client-cert-auth=true \
  --peer-cert-file=/etc/karmada/pki/etcd-peer.crt \
  --peer-key-file=/etc/karmada/pki/etcd-peer.key \
  --peer-trusted-ca-file=/etc/karmada/pki/ca.crt \
  --trusted-ca-file=/etc/karmada/pki/ca.crt \
  --snapshot-count=10000 \
  --initial-advertise-peer-urls https://172.31.209.245:2380 \
  --listen-peer-urls https://172.31.209.245:2380 \
  --listen-client-urls https://172.31.209.245:2379,http://127.0.0.1:2379 \
  --advertise-client-urls https://172.31.209.245:2379 \
  --initial-cluster-token etcd-cluster \
  --initial-cluster karmada-01=https://172.31.209.245:2380,karmada-02=https://172.31.209.246:2380,karmada-03=https://172.31.209.247:2380 \
  --initial-cluster-state new \
  --data-dir=/var/lib/etcd
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Notice:

>The parameters that `karmada-02` `karmada-03` need to change are:
>
>--name
>
>--initial-advertise-peer-urls
>
>--listen-peer-urls
>
>--listen-client-urls
>
>--advertise-client-urls



#### Start etcd cluster

3 servers have to execute.

create etcd storage directory

```bash
mkdir /var/lib/etcd/
chmod 700 /var/lib/etcd
```

start etcd

```bash
systemctl daemon-reload
systemctl enable etcd
systemctl start etcd
systemctl status etcd
```

#### Check etcd cluster status

```bash
etcdctl --cacert=/etc/karmada/pki/ca.crt \
 --cert=/etc/karmada/pki/etcd-server.crt \
 --key=/etc/karmada/pki/etcd-server.key \
 --endpoints 172.31.209.245:2379,172.31.209.246:2379,172.31.209.247:2379 endpoint status --write-out="table"

+---------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
|      ENDPOINT       |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+---------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| 172.31.209.245:2379 | 689151f8cbf4ee95 |   3.5.1 |   20 kB |     false |      false |         2 |          9 |                  9 |        |
| 172.31.209.246:2379 | 5db4dfb6ecc14de7 |   3.5.1 |   20 kB |      true |      false |         2 |          9 |                  9 |        |
| 172.31.209.247:2379 | 7e59eef3c816aa57 |   3.5.1 |   20 kB |     false |      false |         2 |          9 |                  9 |        |
+---------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
```



## Install Karmada APIServer

#### configure nginx

Execute operations at `karmada-01`.

configure load balancing for `karmada apiserver`

```bash
cat > /usr/local/karmada-nginx/conf/nginx.conf <<EOF
worker_processes 2;

events {
    worker_connections  1024;
}

stream {
    upstream backend {
        hash $remote_addr consistent;
        server 172.31.209.245:6443        max_fails=3 fail_timeout=30s;
        server 172.31.209.246:6443        max_fails=3 fail_timeout=30s;
        server 172.31.209.247:6443        max_fails=3 fail_timeout=30s;
    }

    server {
        listen 172.31.209.245:5443;
        proxy_connect_timeout 1s;
        proxy_pass backend;
    }
}
EOF
```

create the `karmada nginx` systemd file

```bash
vi /lib/systemd/system/karmada-nginx.service

[Unit]
Description=The karmada karmada-apiserver nginx proxy server
After=syslog.target network-online.target remote-fs.target nss-lookup.target
Wants=network-online.target

[Service]
Type=forking
ExecStartPre=/usr/local/karmada-nginx/sbin/karmada-nginx -t
ExecStart=/usr/local/karmada-nginx/sbin/karmada-nginx
ExecReload=/usr/local/karmada-nginx/sbin/karmada-nginx -s reload
ExecStop=/bin/kill -s QUIT $MAINPID
PrivateTmp=true
Restart=always
RestartSec=5
StartLimitInterval=0
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

start `karmada nginx`

```bash
systemctl daemon-reload
systemctl enable karmada-nginx
systemctl start karmada-nginx
systemctl status karmada-nginx
```

#### Create systemd file

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

```bash
vi /usr/lib/systemd/system/karmada-apiserver.service
[Unit]
Description=Kubernetes API Service
Documentation=https://github.com/GoogleCloudPlatform/kubernetes
After=network.target
After=etcd.service

[Service]
ExecStart=/usr/local/sbin/kube-apiserver \
  --advertise-address=172.31.209.245 \
  --default-not-ready-toleration-seconds=360 \
  --default-unreachable-toleration-seconds=360 \
  --max-mutating-requests-inflight=2000 \
  --enable-admission-plugins=NodeRestriction \
  --disable-admission-plugins=StorageObjectInUseProtection,ServiceAccount \
  --max-requests-inflight=4000 \
  --default-watch-cache-size=200 \
  --delete-collection-workers=2 \
  --encryption-provider-config=/etc/karmada/encryption-config.yaml \
  --etcd-cafile=/etc/karmada/pki/ca.crt \
  --etcd-certfile=/etc/karmada/pki/karmada-etcd-client.crt \
  --etcd-keyfile=/etc/karmada/pki/karmada-etcd-client.key \
  --etcd-servers=https://172.31.209.245:2379,https://172.31.209.246:2379,https://172.31.209.247:2379 \
  --bind-address=172.31.209.245 \
  --secure-port=6443 \
  --tls-cert-file=/etc/karmada/pki/karmada-apiserver.crt \
  --tls-private-key-file=/etc/karmada/pki/karmada-apiserver.key \
  --insecure-port=0 \
  --audit-webhook-batch-buffer-size=30000 \
  --audit-webhook-batch-max-size=800 \
  --profiling \
  --anonymous-auth=false \
  --client-ca-file=/etc/karmada/pki/ca.crt \
  --enable-bootstrap-token-auth \
  --requestheader-allowed-names="front-proxy-client" \
  --requestheader-client-ca-file=/etc/karmada/pki/ca.crt \
  --requestheader-extra-headers-prefix="X-Remote-Extra-" \
  --requestheader-group-headers=X-Remote-Group \
  --requestheader-username-headers=X-Remote-User \
  --proxy-client-cert-file=/etc/karmada/pki/front-proxy-client.crt \
  --proxy-client-key-file=/etc/karmada/pki/front-proxy-client.key \
  --authorization-mode=Node,RBAC \
  --runtime-config=api/all=true \
  --allow-privileged=true \
  --apiserver-count=3 \
  --event-ttl=168h \
  --service-cluster-ip-range=10.254.0.0/16 \
  --service-node-port-range=10-60060 \
  --enable-swagger-ui=true \
  --logtostderr=true \
  --service-account-issuer=https://kubernetes.default.svc.cluster.local \
  --service-account-key-file=/etc/karmada/pki/sa.pub \
  --service-account-signing-key-file=/etc/karmada/pki/sa.key

Restart=on-failure
RestartSec=5
Type=notify
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

#### Start karmada apiserver

3 servers have to execute.

``` bash
systemctl daemon-reload
systemctl enable karmada-apiserver
systemctl start karmada-apiserver
systemctl status karmada-apiserver
```

#### verify

```bash
# kubectl get cs
Warning: v1 ComponentStatus is deprecated in v1.19+
NAME                 STATUS      MESSAGE                                                                                        ERROR
scheduler            Unhealthy   Get "https://127.0.0.1:10259/healthz": dial tcp 127.0.0.1:10259: connect: connection refused   
controller-manager   Unhealthy   Get "https://127.0.0.1:10257/healthz": dial tcp 127.0.0.1:10257: connect: connection refused   
etcd-0               Healthy     {"health":"true","reason":""}                                                                  
etcd-2               Healthy     {"health":"true","reason":""}                                                                  
etcd-1               Healthy     {"health":"true","reason":""}   
```



## Install kube-controller-manager

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

#### Create systemd file

```bash
vi /usr/lib/systemd/system/kube-controller-manager.service

[Unit]
Description=Kubernetes Controller Manager
Documentation=https://github.com/GoogleCloudPlatform/kubernetes

[Service]
ExecStart=/usr/local/sbin/kube-controller-manager \
  --profiling \
  --cluster-name=karmada \
  --controllers=namespace,garbagecollector,serviceaccount-token\
  --kube-api-qps=1000 \
  --kube-api-burst=2000 \
  --leader-elect \
  --use-service-account-credentials\
  --concurrent-service-syncs=1 \
  --bind-address=0.0.0.0 \
  --address=0.0.0.0 \
  --tls-cert-file=/etc/karmada/pki/kube-controller-manager.crt \
  --tls-private-key-file=/etc/karmada/pki/kube-controller-manager.key \
  --authentication-kubeconfig=/etc/karmada/kube-controller-manager.kubeconfig \
  --client-ca-file=/etc/karmada/pki/ca.crt \
  --requestheader-allowed-names="front-proxy-client" \
  --requestheader-client-ca-file=/etc/karmada/pki/ca.crt \
  --requestheader-extra-headers-prefix="X-Remote-Extra-" \
  --requestheader-group-headers=X-Remote-Group \
  --requestheader-username-headers=X-Remote-User \
  --authorization-kubeconfig=/etc/karmada/kube-controller-manager.kubeconfig \
  --cluster-signing-cert-file=/etc/karmada/pki/ca.crt \
  --cluster-signing-key-file=/etc/karmada/pki/ca.key \
  --experimental-cluster-signing-duration=876000h \
  --feature-gates=RotateKubeletServerCertificate=true \
  --horizontal-pod-autoscaler-sync-period=10s \
  --concurrent-deployment-syncs=10 \
  --concurrent-gc-syncs=30 \
  --node-cidr-mask-size=24 \
  --service-cluster-ip-range=10.254.0.0/16 \
  --pod-eviction-timeout=5m \
  --terminated-pod-gc-threshold=10000 \
  --root-ca-file=/etc/karmada/pki/ca.crt \
  --service-account-private-key-file=/etc/karmada/pki/sa.key \
  --kubeconfig=/etc/karmada/kube-controller-manager.kubeconfig \
  --logtostderr=true \
  --v=4 
Restart=on
RestartSec=5
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target 
```

#### Start kube-controller-manager

```bash
systemctl daemon-reload
systemctl enable kube-controller-manager
systemctl start kube-controller-manager
systemctl status kube-controller-manager
```



## Install karmada-controller-manager

Create  `namespace` and bind the `cluster admin role`. Execute operations at `karmada-01`.

```bash
kubectl create ns karmada-system
kubectl create clusterrolebinding cluster-admin:karmada --clusterrole=cluster-admin --user system:karmada
```



#### Create systemd file

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

```bash
vi /usr/lib/systemd/system/karmada-controller-manager.service

[Unit]
Description=Karmada Controller Manager
Documentation=https://github.com/karmada-io/karmada

[Service]
ExecStart=/usr/local/sbin/karmada-controller-manager \
  --kubeconfig=/etc/karmada/karmada.kubeconfig \
  --bind-address=0.0.0.0 \
  --cluster-status-update-frequency=10s \
  --secure-port=10357 \
  --v=4 
Restart=on
RestartSec=5
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

#### Start karmada-controller-manager

```bash
systemctl daemon-reload
systemctl enable karmada-controller-manager
systemctl start karmada-controller-manager
systemctl status karmada-controller-manager
```



## Install karmada-scheduler

#### Create systemd file

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

```bash
vi /usr/lib/systemd/system/karmada-scheduler.service

[Unit]
Description=Karmada Controller Manager
Documentation=https://github.com/karmada-io/karmada

[Service]
ExecStart=/usr/local/sbin/karmada-scheduler \
  --kubeconfig=/etc/karmada/karmada.kubeconfig \
  --bind-address=0.0.0.0 \
  --feature-gates=Failover=true \
  --enable-scheduler-estimator=true \
  --secure-port=10351 \
  --v=4 
Restart=on
RestartSec=5
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

#### Start karmada-scheduler

```bash
systemctl daemon-reload
systemctl enable karmada-scheduler
systemctl start karmada-scheduler
systemctl status karmada-scheduler
```



## Install karmada-webhook

`karmada-webhook` is different from `scheduler` and `controller-manager`, and its high availability needs to be implemented with `nginx.`

modify the `nginx` configuration and add the following configuration,Execute operations at `karmada-01`.

```bash
cat /usr/local/karmada-nginx/conf/nginx.conf
worker_processes 2;

events {
    worker_connections  1024;
}

stream {
    upstream backend {
        hash  consistent;
        server 172.31.209.245:6443        max_fails=3 fail_timeout=30s;
        server 172.31.209.246:6443        max_fails=3 fail_timeout=30s;
        server 172.31.209.247:6443        max_fails=3 fail_timeout=30s;
    }

    upstream webhook {
        hash  consistent;
        server 172.31.209.245:8443        max_fails=3 fail_timeout=30s;
        server 172.31.209.246:8443        max_fails=3 fail_timeout=30s;
        server 172.31.209.247:8443        max_fails=3 fail_timeout=30s;
    }

    server {
        listen 172.31.209.245:5443;
        proxy_connect_timeout 1s;
        proxy_pass backend;
    }

    server {
        listen 172.31.209.245:4443;
        proxy_connect_timeout 1s;
        proxy_pass webhook;
    }
}
```

Reload `nginx` configuration

```bash
systemctl restart karmada-nginx
```



#### Create systemd file

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

```bash
vi /usr/lib/systemd/system/karmada-webhook.service

[Unit]
Description=Karmada Controller Manager
Documentation=https://github.com/karmada-io/karmada

[Service]
ExecStart=/usr/local/sbin/karmada-webhook \
  --kubeconfig=/etc/karmada/karmada.kubeconfig \
  --bind-address=0.0.0.0 \
  --secure-port=8443 \
  --health-probe-bind-address=:8444 \
  --metrics-bind-address=:8445 \
  --cert-dir=/etc/karmada/pki \
  --v=4 
Restart=on
RestartSec=5
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```



#### Start karmada-webook

```bash
systemctl daemon-reload
systemctl enable karmada-webhook
systemctl start karmada-webhook
systemctl status karmada-webhook
```



## Initialize the karmada resource

Execute operations at `karmada-01`.

#### create crd

```bash
wget https://github.com/karmada-io/karmada/releases/download/v1.0.0/crds.tar.gz
tar -zxvf crds.tar.gz
cd crds/bases 
kubectl apply -f .

cd ../patches/
ca_string=$(sudo cat /etc/karmada/pki/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i "s/{{caBundle}}/${ca_string}/g" webhook_in_resourcebindings.yaml
sed -i "s/{{caBundle}}/${ca_string}/g"  webhook_in_clusterresourcebindings.yaml
sed -i 's/karmada-webhook.karmada-system.svc:443/172.31.209.245:4443/g' webhook_in_resourcebindings.yaml
sed -i 's/karmada-webhook.karmada-system.svc:443/172.31.209.245:4443/g' webhook_in_clusterresourcebindings.yaml

kubectl patch CustomResourceDefinition resourcebindings.work.karmada.io  --patch "$(cat webhook_in_resourcebindings.yaml)"
kubectl patch CustomResourceDefinition clusterresourcebindings.work.karmada.io  --patch "$(cat webhook_in_clusterresourcebindings.yaml)"
```

#### karmada webhook configuration

download the `webhook-configuration.yaml` file.

https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/webhook-configuration.yaml

```bash
sed -i "s/{{caBundle}}/${ca_string}/g"  webhook-configuration.yaml
sed -i 's/karmada-webhook.karmada-system.svc:443/172.31.209.245:4443/g'  webhook-configuration.yaml

kubectl create -f  webhook-configuration.yaml
```



## Install karmada-aggregated-apiserver

Like `karmada-webhook`, use `nginx` for high availability.

modify the `nginx` configuration and add the following configuration,Execute operations at `karmada-01`.

```bash
cat /usr/local/karmada-nginx/conf/nginx.conf
worker_processes 2;

events {
    worker_connections  1024;
}

stream {
    upstream backend {
        hash  consistent;
        server 172.31.209.245:6443        max_fails=3 fail_timeout=30s;
        server 172.31.209.246:6443        max_fails=3 fail_timeout=30s;
        server 172.31.209.247:6443        max_fails=3 fail_timeout=30s;
    }

    upstream webhook {
        hash  consistent;
        server 172.31.209.245:8443        max_fails=3 fail_timeout=30s;
        server 172.31.209.246:8443        max_fails=3 fail_timeout=30s;
        server 172.31.209.247:8443        max_fails=3 fail_timeout=30s;
    }

    upstream aa {
        hash  consistent;
        server 172.31.209.245:7443        max_fails=3 fail_timeout=30s;
        server 172.31.209.246:7443        max_fails=3 fail_timeout=30s;
        server 172.31.209.247:7443        max_fails=3 fail_timeout=30s;
    }

    server {
        listen 172.31.209.245:5443;
        proxy_connect_timeout 1s;
        proxy_pass backend;
    }

    server {
        listen 172.31.209.245:4443;
        proxy_connect_timeout 1s;
        proxy_pass webhook;
    }

    server {
        listen 172.31.209.245:443;
        proxy_connect_timeout 1s;
        proxy_pass aa;
    }
}
```

Reload `nginx` configuration

```bash
systemctl restart karmada-nginx
```

#### Create systemd file

Execute operations at `karmada-01` `karmada-02` `karmada-03`.  Take `karmada-01` as an example.

```bash
vi /usr/lib/systemd/system/karmada-aggregated-apiserver.service

[Unit]
Description=Karmada Controller Manager
Documentation=https://github.com/karmada-io/karmada

[Service]
ExecStart=/usr/local/sbin/karmada-aggregated-apiserver \
  --secure-port=7443 \
  --kubeconfig=/etc/karmada/karmada.kubeconfig \
  --authentication-kubeconfig=/etc/karmada/karmada.kubeconfig  \
  --authorization-kubeconfig=/etc/karmada/karmada.kubeconfig  \
  --karmada-config=/etc/karmada/karmada.kubeconfig  \
  --etcd-servers=https://172.31.209.245:2379,https://172.31.209.246:2379,https://172.31.209.247:2379 \
  --etcd-cafile=/etc/karmada/pki/ca.crt \
  --etcd-certfile=/etc/karmada/pki/karmada-etcd-client.crt \
  --etcd-keyfile=/etc/karmada/pki/karmada-etcd-client.key \
  --tls-cert-file=/etc/karmada/pki/karmada.crt \
  --tls-private-key-file=/etc/karmada/pki/karmada.key \
  --audit-log-path=-  \
  --feature-gates=APIPriorityAndFairness=false  \
  --audit-log-maxage=0  \
  --audit-log-maxbackup=0
Restart=on
RestartSec=5
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

#### Start karmada-aggregated-apiserver

```bash
systemctl daemon-reload
systemctl enable karmada-aggregated-apiserver
systemctl start karmada-aggregated-apiserver
systemctl status karmada-aggregated-apiserver
```



#### Create apiservice

`externalName` is the host name of the node where `nginx` is located (`karmada-01`).

```bash
# vi  apiservice.yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1alpha1.cluster.karmada.io
  labels:
    app: karmada-aggregated-apiserver
    apiserver: "true"
spec:
  insecureSkipTLSVerify: true
  group: cluster.karmada.io
  groupPriorityMinimum: 2000
  service:
    name: karmada-aggregated-apiserver
    namespace: karmada-system
  version: v1alpha1
  versionPriority: 10
---
apiVersion: v1
kind: Service
metadata:
  name: karmada-aggregated-apiserver
  namespace: karmada-system
spec:
  type: ExternalName
  externalName: karmada-01
  

# kubectl create -f  apiservice.yaml
```