#!/usr/bin/env bash
# Copyright 2020 The Karmada Authors.
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

set -o errexit
set -o nounset

# This script deploy karmada control plane to any cluster you want.	REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
mkdir -p "${CERT_DIR}" &>/dev/null ||  mkdir -p "${CERT_DIR}"
rm -f "${CERT_DIR}/*" &>/dev/null ||  rm -f "${CERT_DIR}/*"
KARMADA_APISERVER_SECURE_PORT=${KARMADA_APISERVER_SECURE_PORT:-5443}

# The host cluster name which used to install karmada control plane components.
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
ROOT_CA_FILE=${CERT_DIR}/ca.crt
ROOT_CA_KEY=${CERT_DIR}/ca.key
CFSSL_VERSION="v1.6.5"
LOAD_BALANCER=${LOAD_BALANCER:-false} # whether create a 'LoadBalancer' type service for karmada apiserver
source "${REPO_ROOT}"/hack/util.sh

function usage() {
  echo "This script deploys karmada control plane components to a given cluster."
  echo "Note: This script is an internal script and is not intended used by end-users."
  echo "Usage: hack/deploy-karmada.sh <KUBECONFIG> <CONTEXT_NAME> [HOST_CLUSTER_TYPE]"
  echo "Example: hack/deploy-karmada.sh ~/.kube/config karmada-host local"
  echo -e "Parameters:\n\tKUBECONFIG\t\tYour cluster's kubeconfig that you want to install to"
  echo -e "\tCONTEXT_NAME\t\tThe name of context in 'kubeconfig'"
  echo -e "\tHOST_CLUSTER_TYPE\tThe type of your cluster that will install Karmada. Optional values are 'local' and 'remote',"
  echo -e "\t\t\t\t'local' is default, as that is for the local environment, i.e. for the cluster created by kind."
  echo -e "\t\t\t\tAnd if you want to install karmada to a standalone cluster, set it as 'remote'"
}

# recover the former value of KUBECONFIG
function recover_kubeconfig {
  if [ -n "${CURR_KUBECONFIG+x}" ];then
    export KUBECONFIG="${CURR_KUBECONFIG}"
  else
    unset KUBECONFIG
  fi
}

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

# check config file existence
HOST_CLUSTER_KUBECONFIG=$1
if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi

# check context existence and switch
# backup current kubeconfig before changing KUBECONFIG
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG
fi
export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
HOST_CLUSTER_NAME=$2
if ! kubectl config get-contexts "${HOST_CLUSTER_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${HOST_CLUSTER_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  recover_kubeconfig
  exit 1
fi

HOST_CLUSTER_TYPE=${3:-"local"} # the default of host cluster type is local, i.e. cluster created by kind.

# generate a secret to store the certificates
function generate_cert_related_secrets {
    local karmada_ca
    local karmada_ca_key
    karmada_ca=$(base64 < "${ROOT_CA_FILE}" | tr -d '\r\n')
    karmada_ca_key=$(base64 < "${ROOT_CA_KEY}" | tr -d '\r\n')

    local TEMP_PATH
    TEMP_PATH=$(mktemp -d)
    echo ${TEMP_PATH}

    # 1. generate secret with server cert for each component
    generate_cert_secret karmada-apiserver ${karmada_ca} ${KARMADA_APISERVER_SERVER_CRT} ${KARMADA_APISERVER_SERVER_KEY}
    generate_cert_secret karmada-aggregated-apiserver ${karmada_ca} ${KARMADA_AGGREGATED_APISERVER_SERVER_CRT} ${KARMADA_AGGREGATED_APISERVER_SERVER_KEY}
    generate_cert_secret karmada-metrics-adapter ${karmada_ca} ${KARMADA_METRICS_ADAPTER_SERVER_CRT} ${KARMADA_METRICS_ADAPTER_SERVER_KEY}
    generate_cert_secret karmada-search ${karmada_ca} ${KARMADA_SEARCH_SERVER_CRT} ${KARMADA_SEARCH_SERVER_KEY}
    generate_cert_secret karmada-webhook ${karmada_ca} ${KARMADA_WEBHOOK_SERVER_CRT} ${KARMADA_WEBHOOK_SERVER_KEY}
    generate_cert_secret karmada-interpreter-webhook-example ${karmada_ca} ${SERVER_CRT} ${SERVER_KEY}
    generate_cert_secret karmada-scheduler-estimator ${karmada_ca} ${KARMADA_SCHEDULER_ESTIMATOR_SERVER_CRT} ${KARMADA_SCHEDULER_ESTIMATOR_SERVER_KEY}
    generate_cert_secret etcd ${karmada_ca} ${ETCD_SERVER_CRT} ${ETCD_SERVER_KEY}

    # 2. generate secret with client cert
    generate_cert_secret karmada-apiserver-etcd-client ${karmada_ca} ${KARMADA_APISERVER_ETCD_CLIENT_CRT} ${KARMADA_APISERVER_ETCD_CLIENT_KEY}
    generate_cert_secret karmada-apiserver-front-proxy-client ${karmada_ca} ${FRONT_PROXY_CLIENT_CRT} ${FRONT_PROXY_CLIENT_KEY}
    generate_cert_secret karmada-aggregated-apiserver-etcd-client ${karmada_ca} ${KARMADA_AGGREGATED_APISERVER_ETCD_CLIENT_CRT} ${KARMADA_AGGREGATED_APISERVER_ETCD_CLIENT_KEY}
    generate_cert_secret karmada-search-etcd-client ${karmada_ca} ${KARMADA_SEARCH_ETCD_CLIENT_CRT} ${KARMADA_SEARCH_ETCD_CLIENT_KEY}
    generate_cert_secret etcd-etcd-client ${karmada_ca} ${ETCD_CLIENT_CRT} ${ETCD_CLIENT_KEY}
    generate_cert_secret karmada-scheduler-scheduler-estimator-client ${karmada_ca} ${KARMADA_SCHEDULER_GRPC_CRT} ${KARMADA_SCHEDULER_GRPC_KEY}
    generate_cert_secret karmada-descheduler-scheduler-estimator-client ${karmada_ca} ${KARMADA_DESCHEDULER_GRPC_CRT} ${KARMADA_DESCHEDULER_GRPC_KEY}

    # 3. generate secret with ca cert or sa key
    generate_ca_cert_secret kube-controller-manager ${karmada_ca} ${karmada_ca_key}
    generate_key_pair_secret kube-controller-manager ${SA_PUB} ${SA_KEY}
    generate_key_pair_secret karmada-apiserver ${SA_PUB} ${SA_KEY}

    # 4. generate secret with karmada config for each component using their specific client certs
    generate_config_secret karmada-aggregated-apiserver ${karmada_ca} ${KARMADA_AGGREGATED_APISERVER_CLIENT_CRT} ${KARMADA_AGGREGATED_APISERVER_CLIENT_KEY}
    generate_config_secret karmada-controller-manager ${karmada_ca} ${KARMADA_CONTROLLER_MANAGER_CLIENT_CRT} ${KARMADA_CONTROLLER_MANAGER_CLIENT_KEY}
    generate_config_secret karmada-scheduler ${karmada_ca} ${KARMADA_SCHEDULER_CLIENT_CRT} ${KARMADA_SCHEDULER_CLIENT_KEY}
    generate_config_secret karmada-descheduler ${karmada_ca} ${KARMADA_DESCHEDULER_CLIENT_CRT} ${KARMADA_DESCHEDULER_CLIENT_KEY}
    generate_config_secret karmada-metrics-adapter ${karmada_ca} ${KARMADA_METRICS_ADAPTER_CLIENT_CRT} ${KARMADA_METRICS_ADAPTER_CLIENT_KEY}
    generate_config_secret karmada-search ${karmada_ca} ${KARMADA_SEARCH_CLIENT_CRT} ${KARMADA_SEARCH_CLIENT_KEY}
    generate_config_secret karmada-webhook ${karmada_ca} ${KARMADA_WEBHOOK_CLIENT_CRT} ${KARMADA_WEBHOOK_CLIENT_KEY}
    
    components=(kube-controller-manager karmada-interpreter-webhook-example)
    for component in "${components[@]}"
    do
      generate_config_secret ${component} ${karmada_ca} ${CLIENT_CRT} ${CLIENT_KEY}
    done

    rm -rf "${TEMP_PATH}"
}

function generate_config_secret() {
  local component=$1
  cp "${REPO_ROOT}"/artifacts/deploy/karmada-config-secret.yaml "${TEMP_PATH}"/${component}-config-secret.yaml
  sed -i'' -e "s/\${component}/$1/g" "${TEMP_PATH}"/${component}-config-secret.yaml
  sed -i'' -e "s/\${ca_crt}/$2/g" "${TEMP_PATH}"/${component}-config-secret.yaml
  sed -i'' -e "s/\${client_crt}/$3/g" "${TEMP_PATH}"/${component}-config-secret.yaml
  sed -i'' -e "s/\${client_key}/$4/g" "${TEMP_PATH}"/${component}-config-secret.yaml
  kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/${component}-config-secret.yaml
}

function generate_cert_secret() {
  local name=$1
  cp "${REPO_ROOT}"/artifacts/deploy/karmada-cert-secret.yaml "${TEMP_PATH}"/${name}-cert-secret.yaml
  sed -i'' -e "s/\${name}/$1/g" "${TEMP_PATH}"/${name}-cert-secret.yaml
  sed -i'' -e "s/\${ca_crt}/$2/g" "${TEMP_PATH}"/${name}-cert-secret.yaml
  sed -i'' -e "s/\${tls_crt}/$3/g" "${TEMP_PATH}"/${name}-cert-secret.yaml
  sed -i'' -e "s/\${tls_key}/$4/g" "${TEMP_PATH}"/${name}-cert-secret.yaml
  kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/${name}-cert-secret.yaml
}

function generate_ca_cert_secret() {
  local component=$1
  cp "${REPO_ROOT}"/artifacts/deploy/karmada-ca-cert-secret.yaml "${TEMP_PATH}"/${component}-ca-cert-secret.yaml
  sed -i'' -e "s/\${component}/$1/g" "${TEMP_PATH}"/${component}-ca-cert-secret.yaml
  sed -i'' -e "s/\${ca_crt}/$2/g" "${TEMP_PATH}"/${component}-ca-cert-secret.yaml
  sed -i'' -e "s/\${ca_key}/$3/g" "${TEMP_PATH}"/${component}-ca-cert-secret.yaml
  kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/${component}-ca-cert-secret.yaml
}

function generate_key_pair_secret() {
  local component=$1
  cp "${REPO_ROOT}"/artifacts/deploy/karmada-key-pair-secret.yaml "${TEMP_PATH}"/${component}-key-pair-secret.yaml
  sed -i'' -e "s/\${component}/$1/g" "${TEMP_PATH}"/${component}-key-pair-secret.yaml
  sed -i'' -e "s/\${sa_pub}/$2/g" "${TEMP_PATH}"/${component}-key-pair-secret.yaml
  sed -i'' -e "s/\${sa_key}/$3/g" "${TEMP_PATH}"/${component}-key-pair-secret.yaml
  kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/${component}-key-pair-secret.yaml
}

# install Karmada's APIs
function installCRDs() {
    local context_name=$1
    local crd_path=$2

    kubectl --context="${context_name}" apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

    kubectl --context="${context_name}" apply -k "${crd_path}"/_crds
}

# Use x.x.x.8 IP address, which is the same CIDR with the node address of the Kind cluster,
# as the loadBalancer service address of component karmada-interpreter-webhook-example.
interpreter_webhook_example_service_external_ip_prefix=$(echo $(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}") | awk -F. '{printf "%s.%s.%s",$1,$2,$3}')
interpreter_webhook_example_service_external_ip_address=${interpreter_webhook_example_service_external_ip_prefix}.8

# generate cert
util::cmd_must_exist "openssl"
util::cmd_must_exist_cfssl ${CFSSL_VERSION}
# create CA signers
util::create_signing_certkey "" "${CERT_DIR}" ca karmada '"client auth","server auth"'

karmadaAltNames=("*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1" $(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}") "${interpreter_webhook_example_service_external_ip_address}")
# Define SAN names for each server component
karmada_apiserver_alt_names=("karmada-apiserver.karmada-system.svc.cluster.local" "karmada-apiserver.karmada-system.svc" "localhost" "127.0.0.1" $(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}"))
karmada_aggregated_apiserver_alt_names=("karmada-aggregated-apiserver.karmada-system.svc.cluster.local" "karmada-aggregated-apiserver.karmada-system.svc" "localhost" "127.0.0.1")
karmada_webhook_alt_names=("karmada-webhook.karmada-system.svc.cluster.local" "karmada-webhook.karmada-system.svc" "localhost" "127.0.0.1")
karmada_search_alt_names=("karmada-search.karmada-system.svc.cluster.local" "karmada-search.karmada-system.svc" "localhost" "127.0.0.1")
karmada_metrics_adapter_alt_names=("karmada-metrics-adapter.karmada-system.svc.cluster.local" "karmada-metrics-adapter.karmada-system.svc" "localhost" "127.0.0.1")
karmada_scheduler_estimator_alt_names=("*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1")
etcd_server_alt_names=("etcd.karmada-system.svc.cluster.local" "etcd.karmada-system.svc" "etcd-client.karmada-system.svc.cluster.local" "etcd-client.karmada-system.svc" "localhost" "127.0.0.1")

util::create_certkey "" "${CERT_DIR}" "ca" server server "" "${karmadaAltNames[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" client system:admin system:masters "${karmadaAltNames[@]}"
# Generate server certificates for server components
util::create_certkey "" "${CERT_DIR}" "ca" karmada-apiserver "system:karmada:karmada-apiserver" "" "${karmada_apiserver_alt_names[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-aggregated-apiserver "system:karmada:karmada-aggregated-apiserver" "" "${karmada_aggregated_apiserver_alt_names[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-webhook "system:karmada:karmada-webhook" "" "${karmada_webhook_alt_names[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-search "system:karmada:karmada-search" "" "${karmada_search_alt_names[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-metrics-adapter "system:karmada:karmada-metrics-adapter" "" "${karmada_metrics_adapter_alt_names[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-scheduler-estimator "system:karmada:karmada-scheduler-estimator" "" "${karmada_scheduler_estimator_alt_names[@]}"
util::create_certkey "" "${CERT_DIR}" "ca" etcd-server "system:karmada:etcd-server" "" "${etcd_server_alt_names[@]}"

# Generate client certificates for client components (without SAN)
util::create_certkey "" "${CERT_DIR}" "ca" karmada-apiserver-client "system:karmada:karmada-apiserver" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-aggregated-apiserver-client "system:karmada:karmada-aggregated-apiserver" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-webhook-client "system:karmada:karmada-webhook" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-search-client "system:karmada:karmada-search" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-metrics-adapter-client "system:karmada:karmada-metrics-adapter" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-scheduler-estimator-client "system:karmada:karmada-scheduler-estimator" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-controller-manager-client "system:karmada:karmada-controller-manager" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-scheduler-client "system:karmada:karmada-scheduler" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-descheduler-client "system:karmada:karmada-descheduler" "system:masters"

# ETCD client certificates
util::create_certkey "" "${CERT_DIR}" "ca" karmada-apiserver-etcd-client "system:karmada:karmada-apiserver-etcd-client" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-aggregated-apiserver-etcd-client "system:karmada:karmada-aggregated-apiserver-etcd-client" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-search-etcd-client "system:karmada:karmada-search-etcd-client" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" etcd-client "system:karmada:etcd-etcd-client" ""

# GRPC client certificates
util::create_certkey "" "${CERT_DIR}" "ca" karmada-scheduler-grpc "system:karmada:karmada-scheduler-grpc" "system:masters"
util::create_certkey "" "${CERT_DIR}" "ca" karmada-descheduler-grpc "system:karmada:karmada-descheduler-grpc" "system:masters"

# Front proxy certificates
util::create_certkey "" "${CERT_DIR}" "ca" front-proxy-client "front-proxy-client" ""

# Create service account key pair
util::create_key_pair "" "${CERT_DIR}" "sa"

# create namespace for control plane components
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

SERVER_CRT=$(base64 < "${CERT_DIR}/server.crt" | tr -d '\r\n')
SERVER_KEY=$(base64 < "${CERT_DIR}/server.key" | tr -d '\r\n')
CLIENT_CRT=$(base64 < "${CERT_DIR}/client.crt" | tr -d '\r\n')
CLIENT_KEY=$(base64 < "${CERT_DIR}/client.key" | tr -d '\r\n')

KARMADA_APISERVER_SERVER_CRT=$(base64 < "${CERT_DIR}/karmada-apiserver.crt" | tr -d '\r\n')
KARMADA_APISERVER_SERVER_KEY=$(base64 < "${CERT_DIR}/karmada-apiserver.key" | tr -d '\r\n')
KARMADA_AGGREGATED_APISERVER_SERVER_CRT=$(base64 < "${CERT_DIR}/karmada-aggregated-apiserver.crt" | tr -d '\r\n')
KARMADA_AGGREGATED_APISERVER_SERVER_KEY=$(base64 < "${CERT_DIR}/karmada-aggregated-apiserver.key" | tr -d '\r\n')
KARMADA_WEBHOOK_SERVER_CRT=$(base64 < "${CERT_DIR}/karmada-webhook.crt" | tr -d '\r\n')
KARMADA_WEBHOOK_SERVER_KEY=$(base64 < "${CERT_DIR}/karmada-webhook.key" | tr -d '\r\n')
KARMADA_SEARCH_SERVER_CRT=$(base64 < "${CERT_DIR}/karmada-search.crt" | tr -d '\r\n')
KARMADA_SEARCH_SERVER_KEY=$(base64 < "${CERT_DIR}/karmada-search.key" | tr -d '\r\n')
KARMADA_METRICS_ADAPTER_SERVER_CRT=$(base64 < "${CERT_DIR}/karmada-metrics-adapter.crt" | tr -d '\r\n')
KARMADA_METRICS_ADAPTER_SERVER_KEY=$(base64 < "${CERT_DIR}/karmada-metrics-adapter.key" | tr -d '\r\n')
KARMADA_SCHEDULER_ESTIMATOR_SERVER_CRT=$(base64 < "${CERT_DIR}/karmada-scheduler-estimator.crt" | tr -d '\r\n')
KARMADA_SCHEDULER_ESTIMATOR_SERVER_KEY=$(base64 < "${CERT_DIR}/karmada-scheduler-estimator.key" | tr -d '\r\n')
ETCD_SERVER_CRT=$(base64 < "${CERT_DIR}/etcd-server.crt" | tr -d '\r\n')
ETCD_SERVER_KEY=$(base64 < "${CERT_DIR}/etcd-server.key" | tr -d '\r\n')
ETCD_CLIENT_CRT=$(base64 < "${CERT_DIR}/etcd-client.crt" | tr -d '\r\n')
ETCD_CLIENT_KEY=$(base64 < "${CERT_DIR}/etcd-client.key" | tr -d '\r\n')

KARMADA_APISERVER_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-apiserver-client.crt" | tr -d '\r\n')
KARMADA_APISERVER_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-apiserver-client.key" | tr -d '\r\n')
KARMADA_AGGREGATED_APISERVER_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-aggregated-apiserver-client.crt" | tr -d '\r\n')
KARMADA_AGGREGATED_APISERVER_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-aggregated-apiserver-client.key" | tr -d '\r\n')
KARMADA_WEBHOOK_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-webhook-client.crt" | tr -d '\r\n')
KARMADA_WEBHOOK_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-webhook-client.key" | tr -d '\r\n')
KARMADA_SEARCH_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-search-client.crt" | tr -d '\r\n')
KARMADA_SEARCH_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-search-client.key" | tr -d '\r\n')
KARMADA_METRICS_ADAPTER_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-metrics-adapter-client.crt" | tr -d '\r\n')
KARMADA_METRICS_ADAPTER_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-metrics-adapter-client.key" | tr -d '\r\n')
KARMADA_SCHEDULER_ESTIMATOR_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-scheduler-estimator-client.crt" | tr -d '\r\n')
KARMADA_SCHEDULER_ESTIMATOR_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-scheduler-estimator-client.key" | tr -d '\r\n')
KARMADA_CONTROLLER_MANAGER_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-controller-manager-client.crt" | tr -d '\r\n')
KARMADA_CONTROLLER_MANAGER_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-controller-manager-client.key" | tr -d '\r\n')
KARMADA_SCHEDULER_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-scheduler-client.crt" | tr -d '\r\n')
KARMADA_SCHEDULER_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-scheduler-client.key" | tr -d '\r\n')
KARMADA_DESCHEDULER_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-descheduler-client.crt" | tr -d '\r\n')
KARMADA_DESCHEDULER_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-descheduler-client.key" | tr -d '\r\n')

KARMADA_APISERVER_ETCD_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-apiserver-etcd-client.crt" | tr -d '\r\n')
KARMADA_APISERVER_ETCD_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-apiserver-etcd-client.key" | tr -d '\r\n')
KARMADA_AGGREGATED_APISERVER_ETCD_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-aggregated-apiserver-etcd-client.crt" | tr -d '\r\n')
KARMADA_AGGREGATED_APISERVER_ETCD_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-aggregated-apiserver-etcd-client.key" | tr -d '\r\n')
KARMADA_SEARCH_ETCD_CLIENT_CRT=$(base64 < "${CERT_DIR}/karmada-search-etcd-client.crt" | tr -d '\r\n')
KARMADA_SEARCH_ETCD_CLIENT_KEY=$(base64 < "${CERT_DIR}/karmada-search-etcd-client.key" | tr -d '\r\n')

KARMADA_SCHEDULER_GRPC_CRT=$(base64 < "${CERT_DIR}/karmada-scheduler-grpc.crt" | tr -d '\r\n')
KARMADA_SCHEDULER_GRPC_KEY=$(base64 < "${CERT_DIR}/karmada-scheduler-grpc.key" | tr -d '\r\n')
KARMADA_DESCHEDULER_GRPC_CRT=$(base64 < "${CERT_DIR}/karmada-descheduler-grpc.crt" | tr -d '\r\n')
KARMADA_DESCHEDULER_GRPC_KEY=$(base64 < "${CERT_DIR}/karmada-descheduler-grpc.key" | tr -d '\r\n')

FRONT_PROXY_CLIENT_CRT=$(base64 < "${CERT_DIR}/front-proxy-client.crt" | tr -d '\r\n')
FRONT_PROXY_CLIENT_KEY=$(base64 < "${CERT_DIR}/front-proxy-client.key" | tr -d '\r\n')

SA_PUB=$(base64 < "${CERT_DIR}/sa.pub" | tr -d '\r\n')
SA_KEY=$(base64 < "${CERT_DIR}/sa.key" | tr -d '\r\n')

generate_cert_related_secrets

# deploy karmada etcd
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-etcd.yaml"

# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${ETCD_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

#KARMADA_APISERVER_SERVICE_TYPE is the service type of karmada API Server, For connectivity, it will be different when
# HOST_CLUSTER_TYPE is different. When HOST_CLUSTER_TYPE=local, we will create a ClusterIP type Service. And when
# HOST_CLUSTER_TYPE=remote, we directly use hostNetwork to access Karmada API Server outside the
# karmada-host cluster. Of course, you can create a LoadBalancer service by setting $LOAD_BALANCER=true
KARMADA_APISERVER_SERVICE_TYPE="ClusterIP"

if [ "${HOST_CLUSTER_TYPE}" = "local" ]; then # local mode
  KARMADA_APISERVER_IP=$(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}")
else # remote mode
# KARMADA_APISERVER_IP will be got when Karmada API Server is ready
  if [ "${LOAD_BALANCER}" = true ]; then
    KARMADA_APISERVER_SERVICE_TYPE="LoadBalancer"
  fi
  HOST_CLUSTER_TYPE="remote" # make sure HOST_CLUSTER_TYPE is in local and remote
fi

# deploy karmada apiserver
TEMP_PATH_APISERVER=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH_APISERVER}; }' EXIT
KARMADA_APISERVER_VERSION=${KARMADA_APISERVER_VERSION:-"v1.31.3"}
cp "${REPO_ROOT}"/artifacts/deploy/karmada-apiserver.yaml "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
sed -i'' -e "s/{{service_type}}/${KARMADA_APISERVER_SERVICE_TYPE}/g" "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
sed -i'' -e "s/{{karmada_apiserver_version}}/${KARMADA_APISERVER_VERSION}/g" "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
echo -e "\nApply dynamic rendered apiserver service in ${TEMP_PATH_APISERVER}/karmada-apiserver.yaml."
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml

# Wait for karmada-apiserver to come up before launching the rest of the components.
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${APISERVER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# get Karmada apiserver IP at remote mode
if [ "${HOST_CLUSTER_TYPE}" = "remote" ]; then
  case $KARMADA_APISERVER_SERVICE_TYPE in
    ClusterIP)
      KARMADA_APISERVER_IP=$(kubectl --context="${HOST_CLUSTER_NAME}" get pod -l app=karmada-apiserver -n "${KARMADA_SYSTEM_NAMESPACE}" -o=jsonpath='{.items[0].status.podIP}')
    ;;
    LoadBalancer)
      if util::wait_service_external_ip "${HOST_CLUSTER_NAME}" "karmada-apiserver" "${KARMADA_SYSTEM_NAMESPACE}"; then
        echo "Get service external IP: ${SERVICE_EXTERNAL_IP}, wait to check network connectivity"
        KARMADA_APISERVER_IP=$(util::get_load_balancer_ip) || KARMADA_APISERVER_IP=''
      else
        echo "ERROR: wait service external IP timeout, please check the load balancer IP of service: karmada-apiserver"
        exit 1
      fi
    ;;
  esac
fi

if [[ -n "${KARMADA_APISERVER_IP}" ]]; then
  echo -e "\nKarmada API Server's IP is: ${KARMADA_APISERVER_IP}, host cluster type is: ${HOST_CLUSTER_TYPE}"
else
  echo -e "\nERROR: failed to get Karmada API server IP after creating service 'karmada-apiserver' (host cluster type: ${HOST_CLUSTER_TYPE}), please verify."
  recover_kubeconfig
  exit 1
fi

# write karmada api server config to kubeconfig file
util::append_client_kubeconfig "${HOST_CLUSTER_KUBECONFIG}" "${ROOT_CA_FILE}" "${CERT_DIR}/client.crt" "${CERT_DIR}/client.key" "https://${KARMADA_APISERVER_IP}:${KARMADA_APISERVER_SECURE_PORT}" karmada-apiserver

# deploy kube controller manager
cp "${REPO_ROOT}"/artifacts/deploy/kube-controller-manager.yaml "${TEMP_PATH_APISERVER}"/kube-controller-manager.yaml
sed -i'' -e "s/{{karmada_apiserver_version}}/${KARMADA_APISERVER_VERSION}/g" "${TEMP_PATH_APISERVER}"/kube-controller-manager.yaml
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH_APISERVER}"/kube-controller-manager.yaml
# deploy aggregated-apiserver on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-aggregated-apiserver.yaml"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_AGGREGATION_APISERVER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
# deploy karmada-search on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-search.yaml"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_SEARCH_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
# deploy karmada-metrics-adapter on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-metrics-adapter.yaml"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_METRICS_ADAPTER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# install CRD APIs on karmada apiserver.
if ! kubectl config get-contexts "karmada-apiserver" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: karmada-apiserver not in karmada-apiserver."
  recover_kubeconfig
  exit 1
fi

TEMP_PATH_CRDS=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH_CRDS}; }' EXIT
cp -rf "${REPO_ROOT}"/charts/karmada/_crds "${TEMP_PATH_CRDS}"
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_resourcebindings.yaml"
sed -i'' -e "s/{{name}}/karmada-webhook/g" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_resourcebindings.yaml"
sed -i'' -e "s/{{namespace}}/karmada-system/g" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_resourcebindings.yaml"
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_clusterresourcebindings.yaml"
sed -i'' -e "s/{{name}}/karmada-webhook/g" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_clusterresourcebindings.yaml"
sed -i'' -e "s/{{namespace}}/karmada-system/g" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_clusterresourcebindings.yaml"
installCRDs "karmada-apiserver" "${TEMP_PATH_CRDS}"

# render the caBundle in these apiservice with root ca, then karmada-apiserver can use caBundle to verify corresponding AA's server-cert
TEMP_PATH_APISERVICE=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH_APISERVICE}; }' EXIT
cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-aggregated-apiserver-apiservice.yaml "${TEMP_PATH_APISERVICE}"/karmada-aggregated-apiserver-apiservice.yaml
cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-metrics-adapter-apiservice.yaml "${TEMP_PATH_APISERVICE}"/karmada-metrics-adapter-apiservice.yaml
cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-search-apiservice.yaml "${TEMP_PATH_APISERVICE}"/karmada-search-apiservice.yaml
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_APISERVICE}"/karmada-aggregated-apiserver-apiservice.yaml
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_APISERVICE}"/karmada-metrics-adapter-apiservice.yaml
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_APISERVICE}"/karmada-search-apiservice.yaml

# deploy webhook configurations on karmada apiserver
util::deploy_webhook_configuration "karmada-apiserver" "${ROOT_CA_FILE}" "${REPO_ROOT}/artifacts/deploy/webhook-configuration.yaml"

# deploy APIService on karmada apiserver for karmada-aggregated-apiserver
kubectl --context="karmada-apiserver" apply -f "${TEMP_PATH_APISERVICE}"/karmada-aggregated-apiserver-apiservice.yaml
# make sure apiservice for v1alpha1.cluster.karmada.io is Available
util::wait_apiservice_ready "karmada-apiserver" "${KARMADA_AGGREGATION_APISERVER_LABEL}"

# deploy APIService on karmada apiserver for karmada-search
kubectl --context="karmada-apiserver" apply -f "${TEMP_PATH_APISERVICE}"/karmada-search-apiservice.yaml
# make sure apiservice for v1alpha1.search.karmada.io is Available
util::wait_apiservice_ready "karmada-apiserver" "${KARMADA_SEARCH_LABEL}"

# deploy APIService on karmada apiserver for karmada-metrics-adapter
kubectl --context="karmada-apiserver" apply -f "${TEMP_PATH_APISERVICE}"/karmada-metrics-adapter-apiservice.yaml
# make sure apiservice for karmada metrics adapter is Available
util::wait_apiservice_ready "karmada-apiserver" "${KARMADA_METRICS_ADAPTER_LABEL}"

# grant the admin clusterrole read and write permissions for Karmada resources
kubectl --context="karmada-apiserver" apply -f "${REPO_ROOT}/artifacts/deploy/admin-clusterrole-aggregation.yaml"

# deploy cluster proxy rbac for admin
kubectl --context="karmada-apiserver" apply -f "${REPO_ROOT}/artifacts/deploy/cluster-proxy-admin-rbac.yaml"

# deploy bootstrap token configuration for registering member clusters with PULL mode
karmada_ca=$(base64 < "${ROOT_CA_FILE}" | tr -d '\r\n')
karmada_apiserver_address=https://"${KARMADA_APISERVER_IP}:${KARMADA_APISERVER_SECURE_PORT}"
TEMP_PATH_BOOTSTRAP=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH_BOOTSTRAP}; }' EXIT
cp -rf "${REPO_ROOT}"/artifacts/deploy/bootstrap-token-configuration.yaml "${TEMP_PATH_BOOTSTRAP}"/bootstrap-token-configuration-tmp.yaml
sed -i'' -e "s/{{ca_crt}}/${karmada_ca}/g" "${TEMP_PATH_BOOTSTRAP}"/bootstrap-token-configuration-tmp.yaml
sed -i'' -e "s|{{apiserver_address}}|${karmada_apiserver_address}|g" "${TEMP_PATH_BOOTSTRAP}"/bootstrap-token-configuration-tmp.yaml
kubectl --context="karmada-apiserver" apply -f "${TEMP_PATH_BOOTSTRAP}"/bootstrap-token-configuration-tmp.yaml

# deploy controller-manager on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-controller-manager.yaml"
# deploy scheduler on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-scheduler.yaml"
# deploy descheduler on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-descheduler.yaml"
# deploy webhook on host cluster
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-webhook.yaml"

# make sure all karmada control plane components are ready
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_CONTROLLER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_SCHEDULER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KUBE_CONTROLLER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_WEBHOOK_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
