#!/usr/bin/env bash

# Copyright 2024 The Karmada Authors.
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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

## This script deploy karmada control plane to a cluster using helm chart, and then deploy several member clusters.
## Some member clusters are joined in Push mode, using karmadactl join command.
## Other member clusters are joined in Pull mode, using helm chart of karmada-agent.

# VERSION which version Karmada you want to install
VERSION=${VERSION:-"latest"}

# CHARTDIR the relative path of Karmada charts
CHARTDIR=${CHARTDIR:-"./charts/karmada"}

# NEED_CREATE_KIND_CLUSTER customize whether you need to create clusters by kind, if you have clusters already, please unset these options.
NEED_CREATE_KIND_CLUSTER=${NEED_CREATE_KIND_CLUSTER:-"true"}
NEED_CREATE_KIND_MEMBER_CLUSTER=${NEED_CREATE_KIND_MEMBER_CLUSTER:-"true"}
CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.27.3"}

# KARMADA_HOST_NAME customize your cluster name and context name of karmada-host cluster, if you already has a cluster, replace it by your real name
KARMADA_HOST_NAME=${KARMADA_HOST_NAME:-"karmada-host"}
# KARMADA_HOST_KUBECONFIG customize your kubeconfig of karmada-host cluster, if you already has a cluster, replace it by your real kubeconfig path
KARMADA_HOST_KUBECONFIG=${KARMADA_HOST_KUBECONFIG:-"${HOME}/.kube/karmada-host.config"}

# PUSH_MODE_MEMBERS a map which customizing your push mode member clusters
# the key of the map is the cluster name / context name of your member clusters
# the value of the map is the corresponding kubeconfig path of the member cluster
# if you already has member clusters, replace the key with real name and replace the value with real kubeconfig path
# you can add any number push mode clusters as you expect, just append this map as following examples
declare -A PUSH_MODE_MEMBERS
PUSH_MODE_MEMBERS["member1"]="${HOME}/.kube/member1.config"
PUSH_MODE_MEMBERS["member2"]="${HOME}/.kube/member2.config"

# PULL_MODE_MEMBERS a map which customizing your pull mode member clusters
# the key of the map is the cluster name / context name of your member clusters
# the value of the map is the corresponding kubeconfig path of the member cluster
# if you already has member clusters, replace the key with real name and replace the value with real kubeconfig path
# you can add any number pull mode clusters as you expect, just append this map as following examples
declare -A PULL_MODE_MEMBERS
PULL_MODE_MEMBERS["member3"]="${HOME}/.kube/member3.config"

# IMAGE_FROM customize whether you need fetch images in advance, optional value are as follows:
# pull-in-advance: use 'docker pull' to fetch needed images to local node in advance (in case of network trouble in automatically pulling images)
# make: build karmada images from source code with latest tag and pull images (other than karmada) in advance ( in case of trying latest code)
# empty or any other value: ignored, just pull image by k8s-runtime when needing
IMAGE_FROM=${IMAGE_FROM:-"make"}

# LOAD_IMAGE_IN_ADVANCE if you fetch images in advance and you are using KinD clusters, may be you'd like to load it to KinD in advance too
# if not, please unset this option
LOAD_IMAGE_IN_ADVANCE=${LOAD_IMAGE_IN_ADVANCE:-"true"}

# INSTALL_ESTIMATOR whether install karmada-scheduler-estimator
INSTALL_ESTIMATOR=${INSTALL_ESTIMATOR:-"true"}

# METRICS_SERVER_VERSION the target version of metrics-server
METRICS_SERVER_VERSION=${METRICS_SERVER_VERSION:-"v0.6.3"}

# KIND_VERSION the target version of KinD
KIND_VERSION=${KIND_VERSION:-"v0.22.0"}

# KARMADACTL_VERSION the target version of karmadactl
KARMADACTL_VERSION=${KARMADACTL_VERSION:-"latest"}

echo "########## start installing karmada control plane ##########"

# 1. install KinD if comand not found and you have set NEED_CREATE_KIND_CLUSTER or NEED_CREATE_KIND_MEMBER_CLUSTER
if [[ ${NEED_CREATE_KIND_CLUSTER} || ${NEED_CREATE_KIND_MEMBER_CLUSTER} ]]; then
  if ! util::cmd_exist kind; then
    echo -n "Preparing: installing kind ... "
    util::install_tools "sigs.k8s.io/kind" ${KIND_VERSION}
  fi
fi

# 2. create KinD cluster if you set NEED_CREATE_KIND_CLUSTER
if ${NEED_CREATE_KIND_CLUSTER}; then
  # 1.1 create karmada-host cluster
  kind delete clusters ${KARMADA_HOST_NAME}
  rm -f "${KARMADA_HOST_KUBECONFIG}"
  export CLUSTER_VERSION=${CLUSTER_VERSION}
  "${REPO_ROOT}"/hack/create-cluster.sh ${KARMADA_HOST_NAME} "${KARMADA_HOST_KUBECONFIG}"
fi

# ALL_IMAGES all karmada component images and external images
ALL_IMAGES=$(cat ${CHARTDIR}/values.yaml | grep -C 1 'repository:' | sed 's/*karmadaImageVersion/'${VERSION}'/g' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')
# ALL_EXTERNAL_IMAGES all external images (not include karmada component images)
ALL_EXTERNAL_IMAGES=$(cat ${CHARTDIR}/values.yaml | grep -v 'karmada' | grep -C 1 'repository: ' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')
# AGENT_IMAGE karmada-agent image
AGENT_IMAGE=$(cat ${CHARTDIR}/values.yaml | grep -C 1 'repository: karmada/karmada-agent' | sed 's/*karmadaImageVersion/'${VERSION}'/g' | awk -F ':' '{print $2}' | sed 's/\"//g' | xargs -n3 | awk '{print $1"/"$2":"$3}')

# 3. fetch images in advance is you set IMAGE_FROM
if [ "${IMAGE_FROM}" == "pull-in-advance" ]; then
  ## 3.1 use 'docker pull' to fetch target images to local node in advance
  for img in ${ALL_IMAGES}; do
    docker pull "${img}"
  done
  docker pull registry.k8s.io/metrics-server/metrics-server:${METRICS_SERVER_VERSION}
elif [ "${IMAGE_FROM}" == "make" ]; then
  ## 3.2 build karmada images from source code with latest tag and pull images (other than karmada) in advance
  for img in ${ALL_EXTERNAL_IMAGES}; do
    docker pull "${img}"
  done
  docker pull registry.k8s.io/metrics-server/metrics-server:${METRICS_SERVER_VERSION}

  export VERSION=${VERSION}
  export REGISTRY="docker.io/karmada"
  make images GOOS="linux" .
fi

# 4. load images into KinD karmada-host cluster in advance if you set LOAD_IMAGE_IN_ADVANCE
if [[ ${NEED_CREATE_KIND_CLUSTER} && ${LOAD_IMAGE_IN_ADVANCE} ]]; then
  for img in ${ALL_IMAGES}; do
    kind load docker-image "${img}" --name ${KARMADA_HOST_NAME}
  done
fi

# 5. this script try to deploy karmada-apiserver by host-network
# so, it needs to get host-network ip (node ip) from kube-apiserver, and then add this ip to values.yaml as SANs of certificate
export KUBECONFIG=${KARMADA_HOST_KUBECONFIG}
HOST_IP=$(kubectl get ep kubernetes -o jsonpath='{.subsets[0].addresses[0].ip}')
sed -i'' -e "/localhost/{n;s/      \"127.0.0.1/      \"${HOST_IP}\",\n&/g}" ${CHARTDIR}/values.yaml

# 6. install karmada at karmada-host cluster by helm
# if you want to install some components, do like `--set components={"search,descheduler"}`
helm install karmada -n karmada-system \
  --kubeconfig "${KARMADA_HOST_KUBECONFIG}" \
  --create-namespace \
  --dependency-update \
  --set apiServer.hostNetwork=true,components={"metricsAdapter,search,descheduler"} \
  ${CHARTDIR}

# 7.verify: wait for karmada control plane ready
while [[ $(kubectl get po -A | grep -c karmada-system ) -ne $(kubectl get po -n karmada-system | grep -c Running) ]]; do
  echo "waiting for karmada control plane ready..."; sleep 5;
done
kubectl get po -n karmada-system -o wide

# 8. export kubeconfig of karmada-apiserver to local path
KARMADA_APISERVER_KUBECONFIG="${HOME}/.kube/karmada-apiserver.config"
kubectl get secret -n karmada-system karmada-kubeconfig -o jsonpath={.data.kubeconfig} | base64 -d > "${KARMADA_APISERVER_KUBECONFIG}"
KARMADA_APISERVER_ADDR=$(kubectl get ep karmada-apiserver -n karmada-system | tail -n 1 | awk '{print $2}')
sed -i'' -e "s/karmada-apiserver.karmada-system.svc.*:5443/${KARMADA_APISERVER_ADDR}/g" "${KARMADA_APISERVER_KUBECONFIG}"

echo "########## end installing karmada control plane success ##########"

echo "########## start deploying member clusters ##########"

# 1. create KinD cluster if you set NEED_CREATE_KIND_MEMBER_CLUSTER
if ${NEED_CREATE_KIND_MEMBER_CLUSTER}; then
  ## 1.1. create push mode member clusters by KinD
  for clustername in "${!PUSH_MODE_MEMBERS[@]}"; do
    kind delete clusters "${clustername}"
    rm -f "${PUSH_MODE_MEMBERS[$clustername]}"
    export CLUSTER_VERSION=${CLUSTER_VERSION}
    "${REPO_ROOT}"/hack/create-cluster.sh "${clustername}" "${PUSH_MODE_MEMBERS[$clustername]}"

    kind load docker-image "registry.k8s.io/metrics-server/metrics-server:${METRICS_SERVER_VERSION}" --name "${clustername}"
    "${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${PUSH_MODE_MEMBERS[$clustername]}" "${clustername}"
  done

  ## 1.2. create pull mode member clusters by KinD
  for clustername in "${!PULL_MODE_MEMBERS[@]}"; do
    kind delete clusters "${clustername}"
    rm -f "${PULL_MODE_MEMBERS[$clustername]}"
    export CLUSTER_VERSION=${CLUSTER_VERSION}
    "${REPO_ROOT}"/hack/create-cluster.sh "${clustername}" "${PULL_MODE_MEMBERS[$clustername]}"

    kind load docker-image "registry.k8s.io/metrics-server/metrics-server:${METRICS_SERVER_VERSION}" --name "${clustername}"
    "${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${PULL_MODE_MEMBERS[$clustername]}" "${clustername}"
  done
fi

# 2. load karmada-agent image into pull mode member clusters in advance if you set LOAD_IMAGE_IN_ADVANCE
if [[ ${NEED_CREATE_KIND_MEMBER_CLUSTER} && ${LOAD_IMAGE_IN_ADVANCE} ]]; then
  for clustername in "${!PULL_MODE_MEMBERS[@]}"; do
    kind load docker-image "${AGENT_IMAGE}" --name "${clustername}"
  done
fi

# 3. download karmadactl if command not found
if ! util::cmd_exist karmadactl; then
  echo -n "Preparing: installing karmadactl ... "
  util::install_tools "github.com/karmada-io/karmada/cmd/karmadactl" ${KARMADACTL_VERSION}
fi

declare -A ALL_MEMBERS

# 4. join push mode member clusters by 'karmadactl join' command
for clustername in "${!PUSH_MODE_MEMBERS[@]}"; do
  ALL_MEMBERS[$clustername]=${PUSH_MODE_MEMBERS[$clustername]}
  karmadactl join "${clustername}" --kubeconfig "${KARMADA_APISERVER_KUBECONFIG}" --karmada-context karmada-apiserver --cluster-kubeconfig "${PUSH_MODE_MEMBERS[$clustername]}" --cluster-context "${clustername}"
done

# 5. when you deploy karmada-agent by helm chart, you should manually fill in the cert of karmada-apiserver at values.yaml
# so, it needs to get cert from karmada-apiserver.config for agent
CA_CRT=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep certificate-authority-data | awk -F ': ' '{print $2}' | base64 -d)
AGENT_CRT=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep client-certificate-data | awk -F ': ' '{print $2}' | base64 -d)
AGENT_KEY=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep client-key-data | awk -F ': ' '{print $2}' | base64 -d)

# 6. join pull mode member clusters by helm chart
for clustername in "${!PULL_MODE_MEMBERS[@]}"; do
  clusterConfig=${PULL_MODE_MEMBERS[$clustername]}
  ALL_MEMBERS[$clustername]=$clusterConfig

  MEMBER_APISERVER_ADDR=$(kubectl get ep kubernetes --kubeconfig $clusterConfig --context $clustername | tail -n 1 | awk '{print $2}')

  helm install karmada-agent -n karmada-system \
    --kubeconfig "${clusterConfig}" \
    --kube-context "${clustername}" \
    --create-namespace \
    --dependency-update \
    --set installMode=agent,agent.clusterName="${clustername}",agent.clusterEndpoint=https://"${MEMBER_APISERVER_ADDR}",agent.kubeconfig.server=https://"${KARMADA_APISERVER_ADDR}",agent.kubeconfig.caCrt="${CA_CRT}",agent.kubeconfig.crt="${AGENT_CRT}",agent.kubeconfig.key="${AGENT_KEY}" \
    ${CHARTDIR}
done

echo "########## end deploying member clusters success ##########"

if ${INSTALL_ESTIMATOR}; then
  echo "########## start deploying karmada-scheduler-estimator ##########"

  # as for each member cluster
  for clustername in "${!ALL_MEMBERS[@]}"; do
    # 1. you should manually fill in the cert of member cluster at values.yaml
    # so, it needs to get cert from member cluster kubeconfig for installing karmada-scheduler-estimator
    MEMBER_APISERVER=$(cat ${ALL_MEMBERS[$clustername]} | grep server: | awk -F ': ' '{print $2}')
    MEMBER_CA_CRT=$(cat ${ALL_MEMBERS[$clustername]} | grep certificate-authority-data | awk -F ': ' '{print $2}' | base64 -d)
    MEMBER_CRT=$(cat ${ALL_MEMBERS[$clustername]} | grep client-certificate-data | awk -F ': ' '{print $2}' | base64 -d)
    MEMBER_KEY=$(cat ${ALL_MEMBERS[$clustername]} | grep client-key-data | awk -F ': ' '{print $2}' | base64 -d)

    # 2. install karmada-scheduler-estimator
    helm upgrade -i karmada-scheduler-estimator-${clustername} -n karmada-system \
      --kubeconfig "${KARMADA_HOST_KUBECONFIG}" \
      --set installMode=component,components={"schedulerEstimator"},schedulerEstimator.memberClusters[0].clusterName="${clustername}",schedulerEstimator.memberClusters[0].kubeconfig.server="${MEMBER_APISERVER}",schedulerEstimator.memberClusters[0].kubeconfig.caCrt="${MEMBER_CA_CRT}",schedulerEstimator.memberClusters[0].kubeconfig.crt="${MEMBER_CRT}",schedulerEstimator.memberClusters[0].kubeconfig.key="${MEMBER_KEY}" \
      ${CHARTDIR}
  done

  echo "########## end deploying karmada-scheduler-estimator success ##########"
fi

# verify: wait for member cluster ready and then print member clusters
export KUBECONFIG=${KARMADA_HOST_KUBECONFIG}:${KARMADA_APISERVER_KUBECONFIG}
MEMBERS_NUMBER=${#ALL_MEMBERS[*]}
while [[ "$(kubectl --context karmada-apiserver get clusters -o wide | grep -c "True")" -ne ${MEMBERS_NUMBER} ]]; do
  echo "waiting for member clusters ready..."; sleep 2;
done
kubectl --context karmada-apiserver get cluster -o wide

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  MEMBER_CLUSTER_KUBECONFIGS=$(IFS=: ; echo "${ALL_MEMBERS[*]}")
  echo -e "  export KUBECONFIG=${KARMADA_HOST_KUBECONFIG}:${KARMADA_APISERVER_KUBECONFIG}:${MEMBER_CLUSTER_KUBECONFIGS}"
  echo "Please use 'kubectl config use-context ${KARMADA_HOST_NAME}/karmada-apiserver' to switch the host and control plane cluster."
  MEMBER_CLUSTER_NAMES=$(IFS=: ; echo "${!ALL_MEMBERS[*]}")
  echo "Please use 'kubectl config use-context ${MEMBER_CLUSTER_NAMES}' to switch to the different member cluster."
}

print_success
