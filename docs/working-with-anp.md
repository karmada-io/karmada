# Deploy apiserver-network-proxy (ANP)

## Purpose

For a member cluster that joins Karmada in the pull mode, you need to provide a method to connect the network between the Karmada control plane and the member cluster, so that karmada-aggregated-apiserver can access this member cluster.

Deploying ANP to achieve appeal is one of the methods. This article describes how to deploy ANP in Karmada.

## Environment

Karmada can be deployed using the kind tool.

You can directly use `hack/local-up-karmada.sh` to deploy Karmada.

## Actions

### Step 1: Download code

To facilitate demonstration, the code is modified based on ANP v0.0.24 to support access to the front server through HTTP. Here is the code repository address: https://github.com/mrlihanbo/apiserver-network-proxy/tree/v0.0.24/dev.

```shell
git clone -b v0.0.24/dev https://github.com/mrlihanbo/apiserver-network-proxy.git
cd apiserver-network-proxy/
```

### Step 2: Compile images

Compile the proxy-server and proxy-agent images.

```shell
docker build . --build-arg ARCH=amd64 -f artifacts/images/agent-build.Dockerfile -t swr.ap-southeast-1.myhuaweicloud.com/karmada/proxy-agent:0.0.24

docker build . --build-arg ARCH=amd64 -f artifacts/images/server-build.Dockerfile -t swr.ap-southeast-1.myhuaweicloud.com/karmada/proxy-server:0.0.24
```

### Step 3: Generate a certificate

Run the command to check the IP address of karmada-host-control-plane:

```shell
docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' karmada-host-control-plane
```

Run the `make certs` command to generate a certificate and specify PROXY_SERVER_IP as the IP address obtained in the preceding command.

```shell
make certs PROXY_SERVER_IP=x.x.x.x
```

The certificate is generated in the `certs` folder.

### Step 4: Deploy proxy-server

Save the `proxy-server.yaml` file in the root directory of the ANP code repository.

<details>
<summary>unfold me to see the yaml</summary>

```yaml
# proxy-server.yaml

apiVersion: apps/v1
kind: Deployment
metadata:
  name: proxy-server
  namespace: karmada-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: proxy-server
  template:
    metadata:
      labels:
        app: proxy-server
    spec:
      containers:
      - command:
        - /proxy-server
        args:
          - --health-port=8092
          - --cluster-ca-cert=/var/certs/server/cluster-ca-cert.crt
          - --cluster-cert=/var/certs/server/cluster-cert.crt 
          - --cluster-key=/var/certs/server/cluster-key.key
          - --mode=http-connect 
          - --proxy-strategies=destHost 
          - --server-ca-cert=/var/certs/server/server-ca-cert.crt
          - --server-cert=/var/certs/server/server-cert.crt 
          - --server-key=/var/certs/server/server-key.key
        image: swr.ap-southeast-1.myhuaweicloud.com/karmada/proxy-server:0.0.24
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 8092
            scheme: HTTP
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 60
        name: proxy-server
        volumeMounts:
        - mountPath: /var/certs/server
          name: cert
      restartPolicy: Always
      hostNetwork: true
      volumes:
      - name: cert
        secret:
          secretName: proxy-server-cert
---
apiVersion: v1
kind: Secret
metadata:
  name: proxy-server-cert
  namespace: karmada-system
type: Opaque
data:
  server-ca-cert.crt: |
    {{server_ca_cert}}
  server-cert.crt: |
    {{server_cert}}
  server-key.key: |
    {{server_key}}
  cluster-ca-cert.crt: |
    {{cluster_ca_cert}}
  cluster-cert.crt: |
    {{cluster_cert}}
  cluster-key.key: |
    {{cluster_key}}
```

</details>

Save the `replace-proxy-server.sh` file in the root directory of the ANP code repository.

<details>
<summary>unfold me to see the shell</summary>

```shell
#!/bin/bash

cert_yaml=proxy-server.yaml

SERVER_CA_CERT=$(cat certs/frontend/issued/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{server_ca_cert}}/${SERVER_CA_CERT}/g" ${cert_yaml}

SERVER_CERT=$(cat certs/frontend/issued/proxy-frontend.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{server_cert}}/${SERVER_CERT}/g" ${cert_yaml}

SERVER_KEY=$(cat certs/frontend/private/proxy-frontend.key | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{server_key}}/${SERVER_KEY}/g" ${cert_yaml}

CLUSTER_CA_CERT=$(cat certs/agent/issued/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{cluster_ca_cert}}/${CLUSTER_CA_CERT}/g" ${cert_yaml}

CLUSTER_CERT=$(cat certs/agent/issued/proxy-frontend.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{cluster_cert}}/${CLUSTER_CERT}/g" ${cert_yaml}


CLUSTER_KEY=$(cat certs/agent/private/proxy-frontend.key | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{cluster_key}}/${CLUSTER_KEY}/g" ${cert_yaml}
```

</details>

Run the following commands to run the script:

```shell
chmod +x replace-proxy-server.sh
bash replace-proxy-server.sh
```

Deploy the proxy-server on the Karmada control plane:

```shell
kind load docker-image swr.ap-southeast-1.myhuaweicloud.com/karmada/proxy-server:0.0.24 --name karmada-host
export KUBECONFIG=/root/.kube/karmada.config
kubectl --context=karmada-host apply -f proxy-server.yaml
```

### Step 5: Deploy proxy-agent

Save the `proxy-agent.yaml` file in the root directory of the ANP code repository.

<details>
<summary>unfold me to see the yaml</summary>

```yaml
# proxy-agent.yaml

apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: proxy-agent
  name: proxy-agent
  namespace: karmada-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: proxy-agent
  template:
    metadata:
      labels:
        app: proxy-agent
    spec:
      containers:
        - command:
            - /proxy-agent
          args:
            - '--ca-cert=/var/certs/agent/ca.crt'
            - '--agent-cert=/var/certs/agent/proxy-agent.crt'
            - '--agent-key=/var/certs/agent/proxy-agent.key'
            - '--proxy-server-host={{proxy_server_addr}}'
            - '--proxy-server-port=8091'
            - '--agent-identifiers=host={{identifiers}}'
          image: swr.ap-southeast-1.myhuaweicloud.com/karmada/proxy-agent:0.0.24
          imagePullPolicy: IfNotPresent
          name: proxy-agent
          livenessProbe:
            httpGet:
              scheme: HTTP
              port: 8093
              path: /healthz
            initialDelaySeconds: 15
            timeoutSeconds: 60
          volumeMounts:
            - mountPath: /var/certs/agent
              name: cert
      volumes:
        - name: cert
          secret:
            secretName: proxy-agent-cert
---
apiVersion: v1
kind: Secret
metadata:
  name: proxy-agent-cert
  namespace: karmada-system
type: Opaque
data:
  ca.crt: |
    {{proxy_agent_ca_crt}}
  proxy-agent.crt: |
    {{proxy_agent_crt}}
  proxy-agent.key: |
    {{proxy_agent_key}}
```

</details>

Save the `replace-proxy-agent.sh` file in the root directory of the ANP code repository.

<details>
<summary>unfold me to see the shell</summary>

```shell
#!/bin/bash

cert_yaml=proxy-agent.yaml

karmada_controlplan_addr=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' karmada-host-control-plane)
member3_cluster_addr=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' member3-control-plane)
sed -i'' -e "s/{{proxy_server_addr}}/${karmada_controlplan_addr}/g" ${cert_yaml}
sed -i'' -e "s/{{identifiers}}/${member3_cluster_addr}/g" ${cert_yaml}

PROXY_AGENT_CA_CRT=$(cat certs/agent/issued/ca.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{proxy_agent_ca_crt}}/${PROXY_AGENT_CA_CRT}/g" ${cert_yaml}

PROXY_AGENT_CRT=$(cat certs/agent/issued/proxy-agent.crt | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{proxy_agent_crt}}/${PROXY_AGENT_CRT}/g" ${cert_yaml}

PROXY_AGENT_KEY=$(cat certs/agent/private/proxy-agent.key | base64 | tr "\n" " "|sed s/[[:space:]]//g)
sed -i'' -e "s/{{proxy_agent_key}}/${PROXY_AGENT_KEY}/g" ${cert_yaml}
```

</details>

Run the following commands to run the script:

```shell
chmod +x replace-proxy-agent.sh
bash replace-proxy-agent.sh
```

Deploy the proxy-agent in the pull mode for a member cluster (in this example, the `member3` cluster is in the pull mode.):

```shell
kind load docker-image swr.ap-southeast-1.myhuaweicloud.com/karmada/proxy-agent:0.0.24 --name member3
kubectl --kubeconfig=/root/.kube/members.config --context=member3 apply -f proxy-agent.yaml
```

**The ANP deployment is complete now.**

### Step 6: Add command flags for the karmada-agent deployment

After deploying the ANP deployment, you need to add extra command flags `--cluster-api-endpoint` and `--proxy-server-address` for the `karmada-agent` deployment in the `member3` cluster.

Where `--cluster-api-endpoint` is the APIEndpoint of the cluster. You can obtain it from the KubeConfig file of the `member3` cluster.

Where `--proxy-server-address` is the address of the proxy server that is used to proxy the cluster. In current case, you can set `--proxy-server-address` to `http://<karmada_controlplan_addr>:8088`. Get `karmada_controlplan_addr` value through the following command:

```shell
docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' karmada-host-control-plane
```

Set port `8088` by modifying the code in ANP: https://github.com/mrlihanbo/apiserver-network-proxy/blob/v0.0.24/dev/cmd/server/app/server.go#L267. You can also modify it to a different value.
