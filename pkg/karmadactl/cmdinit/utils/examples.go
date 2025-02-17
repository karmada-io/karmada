/*
Copyright 2021 The Karmada Authors.

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

package utils

import (
	"fmt"

	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
)

const (
	// https://github.com/karmada-io/karmada/blob/master/artifacts/agent
	karmadaAgent = `---
apiVersion: v1
kind: Namespace
metadata:
  name: karmada-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: karmada-agent
rules:
  - apiGroups: ['*']
    resources: ['*']
    verbs: ['*']
  - nonResourceURLs: ['*']
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: karmada-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: karmada-agent
subjects:
  - kind: ServiceAccount
    name: karmada-agent-sa
    namespace: karmada-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: karmada-agent-sa
  namespace: karmada-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-agent
  namespace: karmada-system
  labels:
    app: karmada-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: karmada-agent
  template:
    metadata:
      labels:
        app: karmada-agent
    spec:
      serviceAccountName: karmada-agent-sa
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-agent
          image: docker.io/karmada/karmada-agent:latest
          command:
            - /bin/karmada-agent
            - --karmada-kubeconfig=/etc/kubeconfig/karmada-kubeconfig
            - --karmada-context=%s
            - --cluster-name={member_cluster_name}
            - --cluster-api-endpoint={member_cluster_api_endpoint}
            - --cluster-status-update-frequency=10s
            - --health-probe-bind-address=0.0.0.0:10357
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
              mountPath: /etc/kubeconfig
      volumes:
        - name: kubeconfig
          secret:
            secretName: karmada-kubeconfig`

	estimator = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: karmada-scheduler-estimator-{{member_cluster_name}}
  namespace: karmada-system
  labels:
    cluster: {{member_cluster_name}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: karmada-scheduler-estimator-{{member_cluster_name}}
  template:
    metadata:
      labels:
        app: karmada-scheduler-estimator-{{member_cluster_name}}
    spec:
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
      containers:
        - name: karmada-scheduler-estimator
          image: docker.io/karmada/karmada-scheduler-estimator:latest
          imagePullPolicy: IfNotPresent
          command:
            - /bin/karmada-scheduler-estimator
            - --kubeconfig=/etc/{{member_cluster_name}}-kubeconfig
            - --cluster-name={{member_cluster_name}}
            - --metrics-bind-address=0.0.0.0:8080
            - --health-probe-bind-address=0.0.0.0:10351
          volumeMounts:
            - name: member-kubeconfig
              subPath: {{member_cluster_name}}-kubeconfig
              mountPath: /etc/{{member_cluster_name}}-kubeconfig
      volumes:
        - name: member-kubeconfig
          secret:
            secretName: {{member_cluster_name}}-kubeconfig
---
apiVersion: v1
kind: Service
metadata:
  name: karmada-scheduler-estimator-{{member_cluster_name}}
  namespace: karmada-system
  labels:
    cluster: {{member_cluster_name}}
spec:
  selector:
    app: karmada-scheduler-estimator-{{member_cluster_name}}
  ports:
    - protocol: TCP
      port: 10352
      targetPort: 10352`
)

// GenExamples Generate sample files
func GenExamples(path, parentCommand string) {
	karmadaAgentStr := fmt.Sprintf(karmadaAgent, options.ClusterName)
	if err := BytesToFile(path, "karmada-agent.yaml", []byte(karmadaAgentStr)); err != nil {
		klog.Warning(err)
	}

	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-scheduler-estimator.yaml
	if err := BytesToFile(path, "karmada-scheduler-estimator.yaml", []byte(estimator)); err != nil {
		klog.Warning(err)
	}

	fmt.Printf(`
------------------------------------------------------------------------------------------------------
 █████   ████   █████████   ███████████   ██████   ██████   █████████   ██████████     █████████
░░███   ███░   ███░░░░░███ ░░███░░░░░███ ░░██████ ██████   ███░░░░░███ ░░███░░░░███   ███░░░░░███
 ░███  ███    ░███    ░███  ░███    ░███  ░███░█████░███  ░███    ░███  ░███   ░░███ ░███    ░███
 ░███████     ░███████████  ░██████████   ░███░░███ ░███  ░███████████  ░███    ░███ ░███████████
 ░███░░███    ░███░░░░░███  ░███░░░░░███  ░███ ░░░  ░███  ░███░░░░░███  ░███    ░███ ░███░░░░░███
 ░███ ░░███   ░███    ░███  ░███    ░███  ░███      ░███  ░███    ░███  ░███    ███  ░███    ░███
 █████ ░░████ █████   █████ █████   █████ █████     █████ █████   █████ ██████████   █████   █████
░░░░░   ░░░░ ░░░░░   ░░░░░ ░░░░░   ░░░░░ ░░░░░     ░░░░░ ░░░░░   ░░░░░ ░░░░░░░░░░   ░░░░░   ░░░░░
------------------------------------------------------------------------------------------------------
Karmada is installed successfully.

Register Kubernetes cluster to Karmada control plane.

Register cluster with 'Push' mode

Step 1: Use "%[2]s join" command to register the cluster to Karmada control plane. --cluster-kubeconfig is kubeconfig of the member cluster.
(In karmada)~# MEMBER_CLUSTER_NAME=$(cat ~/.kube/config  | grep current-context | sed 's/: /\n/g'| sed '1d'| tr -d "\"'")
(In karmada)~# %[2]s --kubeconfig %[1]s/karmada-apiserver.config  join ${MEMBER_CLUSTER_NAME} --cluster-kubeconfig=$HOME/.kube/config

Step 2: Show members of karmada
(In karmada)~# kubectl --kubeconfig %[1]s/karmada-apiserver.config get clusters


Register cluster with 'Pull' mode

Step 1: Create bootstrap token and generate the '%[2]s register' command which will be used later.
~# %[2]s token create --print-register-command --kubeconfig=%[1]s/karmada-apiserver.config
This command will generate a registration command similar to:

%[2]s register 172.18.0.5:5443 --token t8xfio.640u9gp9obc72v5d --discovery-token-ca-cert-hash sha256:9cfa542ff48f43793d1816b1dd0a78ad574e349d8f6e005e6e32e8ab528e4244

Step 2: Use the output from Step 1 to register the cluster to the Karmada control plane. 
You need to specify the target member cluster by flag '--kubeconfig'
~# %[2]s register 172.18.0.5:5443 --token t8xfio.640u9gp9obc72v5d --discovery-token-ca-cert-hash sha256:9cfa542ff48f43793d1816b1dd0a78ad574e349d8f6e005e6e32e8ab528e4244 --kubeconfig=<path-to-member-cluster-kubeconfig>

Step 3: Show members of Karmada.
~# %[2]s --kubeconfig=%[1]s/karmada-apiserver.config get clusters

The %[2]s register command has several optional parameters for setting the properties of the member cluster. For more details, run:

~# %[2]s register --help

`, path, parentCommand)
}
