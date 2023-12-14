#!/usr/bin/env bash

# Copyright 2021 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


## This script deploy karmada control plane to a cluster using helm chart, and then deploy several member clusters.
## Some member clusters are joined in Push mode, using karmadactl join command.
## Other member clusters are joined in Pull mode, using helm chart of karmada-agent.

# NEED_CREATE_KIND_CLUSTER customize whether you need to create clusters by kind, if you have clusters already, please unset this option
NEED_CREATE_KIND_CLUSTER=true

# IMAGE_FROM customize whether you need fetch images in advance, optional value are as follows:
# pull-in-advance: use 'docker pull' to fetch needed images to local node in advance (in case of trouble in automatically pulling images)
# make: build karmada images from source code with latest tag and pull images (other than karmada) in advance ( in case of trying latest code)
# empty or any other value: ignored, just pull image by k8s-runtime when needing
IMAGE_FROM="pull-in-advance"

# LOAD_IMAGE_IN_ADVANCE if you fetch images in advance and you are using KinD clusters, may be you'd like to load it to KinD in advance too
# if not, please unset this option
LOAD_IMAGE_IN_ADVANCE=true

# KARMADA_HOST_NAME customize your cluster name and context name of karmada-host cluster, if you already has a cluster, replace it by your real name
KARMADA_HOST_NAME="karmada-host"
# KARMADA_HOST_KUBECONFIG customize your kubeconfig of karmada-host cluster, if you already has a cluster, replace it by your real kubeconfig path
KARMADA_HOST_KUBECONFIG="${HOME}/.kube/karmada-host.config"

# PUSH_MODE_MEMBERS a map which customizing your push mode member clusters
# the key of the map is the cluster name / context name of your member clusters
# the value of the map is the corresponding kubeconfig path of the member cluster
# if you already has member clusters, replace the key with real name and replace the value with real kubeconfig path
# you can add any number push mode clusters as you expect, just append this map as following examples
declare -A PUSH_MODE_MEMBERS
PUSH_MODE_MEMBERS["member1"]="${HOME}/.kube/members.config"
PUSH_MODE_MEMBERS["member3"]="${HOME}/.kube/members.config"

# PULL_MODE_MEMBERS a map which customizing your pull mode member clusters
# the key of the map is the cluster name / context name of your member clusters
# the value of the map is the corresponding kubeconfig path of the member cluster
# if you already has member clusters, replace the key with real name and replace the value with real kubeconfig path
# you can add any number pull mode clusters as you expect, just append this map as following examples
declare -A PULL_MODE_MEMBERS
PULL_MODE_MEMBERS["member2"]="${HOME}/.kube/members.config"

echo "########## start installing karmada control plane ##########"

# 1. create KinD cluster if you set NEED_CREATE_KIND_CLUSTER
if ${NEED_CREATE_KIND_CLUSTER}; then
  # 1.1 create karmada-host cluster
  kind delete clusters karmada-host
  rm -rf ~/.karmada "${KARMADA_HOST_KUBECONFIG}"
  hack/create-cluster.sh ${KARMADA_HOST_NAME} "${KARMADA_HOST_KUBECONFIG}"
fi

# 2. fetch images in advance is you set IMAGE_FROM
if [ "${IMAGE_FROM}" == "pull-in-advance" ]; then
  ## 2.1 use 'docker pull' to fetch target images to local node in advance
  imgs=$(cat charts/karmada/values.yaml | grep -C 1 'repository:' | sed 's/*karmadaImageVersion/latest/g' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')
  for img in ${imgs}; do
    docker pull "${img}"
  done
elif [ "${IMAGE_FROM}" == "make" ]; then
  ## 2.2 build karmada images from source code with latest tag and pull images (other than karmada) in advance
  imgs=$(cat charts/karmada/values.yaml | grep -v 'karmada' | grep -C 1 'repository: ' | sed 's/*karmadaImageVersion/latest/g' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')
  for img in ${imgs}; do
    docker pull "${img}"
  done

  export VERSION="latest"
  export REGISTRY="docker.io/karmada"
  make images GOOS="linux" .
fi

# 3. load images into KinD karmada-host cluster in advance if you set LOAD_IMAGE_IN_ADVANCE
if ${LOAD_IMAGE_IN_ADVANCE}; then
  imgs=$(cat charts/karmada/values.yaml | grep -C 1 'repository:' | sed 's/*karmadaImageVersion/latest/g' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')
  for img in ${imgs}; do
    kind load docker-image "${img}" --name ${KARMADA_HOST_NAME}
  done
fi

# 4. this script try to deploy karmada-apiserver by host-network
# so, it needs to get host-network ip (node ip) from kube-apiserver, and then add this ip to values.yaml as SANs of certificate
export KUBECONFIG=${KARMADA_HOST_KUBECONFIG}
HOST_IP=$(kubectl get ep kubernetes -o jsonpath='{.subsets[0].addresses[0].ip}')
sed -i'' -e "/localhost/p; s/localhost/${HOST_IP}/" charts/karmada/values.yaml

# 5. install karmada at karmada-host cluster by helm
helm install karmada -n karmada-system \
        --kubeconfig "${KARMADA_HOST_KUBECONFIG}" \
        --create-namespace \
        --dependency-update \
        --set apiServer.hostNetwork=true \
        ./charts/karmada

# 6. export kubeconfig of karmada-apiserver to local path
KARMADA_APISERVER_KUBECONFIG="${HOME}/.kube/karmada-apiserver.config"
kubectl get secret -n karmada-system karmada-kubeconfig -o jsonpath={.data.kubeconfig} | base64 -d > "${KARMADA_APISERVER_KUBECONFIG}"
KARMADA_APISERVER_ADDR=$(kubectl get ep karmada-apiserver -n karmada-system | tail -n 1 | awk '{print $2}')
sed -i'' -e "s/karmada-apiserver.karmada-system.svc.*:5443/${KARMADA_APISERVER_ADDR}/g" "${KARMADA_APISERVER_KUBECONFIG}"

echo "########## end installing karmada control plane success ##########"


echo "########## start deploying member clusters ##########"

# 1. create KinD cluster if you set NEED_CREATE_KIND_CLUSTER
if ${NEED_CREATE_KIND_CLUSTER}; then
  ## 1.1. create push mode member clusters by KinD
  for clustername in "${!PUSH_MODE_MEMBERS[@]}"; do
    kind delete clusters "${clustername}"
    hack/create-cluster.sh "${clustername}" "${PUSH_MODE_MEMBERS[$clustername]}"
  done

  ## 1.2. create pull mode member clusters by KinD
  for clustername in "${!PULL_MODE_MEMBERS[@]}"; do
      kind delete clusters "${clustername}"
      hack/create-cluster.sh "${clustername}" "${PULL_MODE_MEMBERS[$clustername]}"
  done
fi

# 2. load karmada-agent image into pull mode member clusters in advance if you set LOAD_IMAGE_IN_ADVANCE
if ${LOAD_IMAGE_IN_ADVANCE}; then
  agentImage=$(cat charts/karmada/values.yaml | grep -C 1 'repository: karmada/karmada-agent' | sed 's/*karmadaImageVersion/latest/g' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')
  for clustername in "${!PULL_MODE_MEMBERS[@]}"; do
    kind load docker-image "${agentImage}" --name "${clustername}"
  done
fi

# 3. download karmadactl if not exist
if ! which karmadactl >/dev/null 2>&1; then
  GO111MODULE=on go install "github.com/karmada-io/karmada/cmd/karmadactl"
  GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
  alias karmadactl='${GOPATH}/bin/karmadactl'
fi

# 4. join push mode member clusters by 'karmadactl join' command
for clustername in "${!PUSH_MODE_MEMBERS[@]}"; do
  karmadactl join "${clustername}" --kubeconfig "${KARMADA_APISERVER_KUBECONFIG}" --karmada-context karmada-apiserver --cluster-kubeconfig "${PUSH_MODE_MEMBERS[$clustername]}" --cluster-context "${clustername}"
done

# 5. when you deploy karmada-agent by helm chart, you should manually fill in the cert of karmada-apiserver at values.yaml
# so, it needs to get cert from karmada-apiserver.config for agent
CA_CRT=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep certificate-authority-data | awk -F ': ' '{print $2}' | base64 -d)
AGENT_CRT=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep client-certificate-data | awk -F ': ' '{print $2}' | base64 -d)
AGENT_KEY=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep client-key-data | awk -F ': ' '{print $2}' | base64 -d)

# 6. join pull mode member clusters by helm chart
for clustername in "${!PULL_MODE_MEMBERS[@]}"; do
  helm install karmada-agent -n karmada-system \
          --kubeconfig "${PULL_MODE_MEMBERS[$clustername]}" \
          --kube-context "${clustername}" \
          --create-namespace \
          --dependency-update \
          --set installMode=agent,agent.clusterName="${clustername}",agent.kubeconfig.server=https://"${KARMADA_APISERVER_ADDR}",agent.kubeconfig.caCrt="${CA_CRT}",agent.kubeconfig.crt="${AGENT_CRT}",agent.kubeconfig.key="${AGENT_KEY}" \
          ./charts/karmada
done

echo "########## end deploying member clusters success ##########"

# merge karmada-host.config and karmada-apiserver.config into ${KARMADA_MERGE_KUBECONFIG}, keep the same with other installation method
KARMADA_MERGE_KUBECONFIG="${HOME}/.kube/karmada.config"
export KUBECONFIG="${KARMADA_HOST_KUBECONFIG}":"${KARMADA_APISERVER_KUBECONFIG}"
kubectl config view --flatten > "${KARMADA_MERGE_KUBECONFIG}"

# verify: wait for member cluster ready and then print member clusters
MEMBERS_NUMBER=$(( ${#PUSH_MODE_MEMBERS[*]} + ${#PULL_MODE_MEMBERS[*]} ))
while [[ "$(kubectl --context karmada-apiserver get clusters -o wide | grep -c "True")" -ne ${MEMBERS_NUMBER} ]]; do
  echo "waiting for member clusters ready..."; sleep 2;
done
kubectl --context karmada-apiserver get cluster -o wide
