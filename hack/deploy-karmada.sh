#!/usr/bin/env bash

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
ROOT_CA_FILE=${CERT_DIR}/server-ca.crt
CFSSL_VERSION="v1.5.0"
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
if ! kubectl config use-context "${HOST_CLUSTER_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to use context: '${HOST_CLUSTER_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  recover_kubeconfig
  exit 1
fi

HOST_CLUSTER_TYPE=${3:-"local"} # the default of host cluster type is local, i.e. cluster created by kind.

# generate a secret to store the certificates
function generate_cert_secret {
    local karmada_ca
    karmada_ca=$(base64 "${ROOT_CA_FILE}" | tr -d '\r\n')

    local TEMP_PATH
    TEMP_PATH=$(mktemp -d)

    cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-cert-secret.yaml "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    cp -rf "${REPO_ROOT}"/artifacts/deploy/secret.yaml "${TEMP_PATH}"/secret-tmp.yaml
    cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-webhook-cert-secret.yaml "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml

    sed -i'' -e "s/{{ca_crt}}/${karmada_ca}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    sed -i'' -e "s/{{client_cer}}/${KARMADA_CRT}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    sed -i'' -e "s/{{client_key}}/${KARMADA_KEY}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml

    sed -i'' -e "s/{{front_proxy_ca_crt}}/${FRONT_PROXY_CA_CRT}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    sed -i'' -e "s/{{front_proxy_client_crt}}/${FRONT_PROXY_CLIENT_CRT}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    sed -i'' -e "s/{{front_proxy_client_key}}/${FRONT_PROXY_CLIENT_KEY}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml

    sed -i'' -e "s/{{ca_crt}}/${karmada_ca}/g" "${TEMP_PATH}"/secret-tmp.yaml
    sed -i'' -e "s/{{client_cer}}/${KARMADA_CRT}/g" "${TEMP_PATH}"/secret-tmp.yaml
    sed -i'' -e "s/{{client_key}}/${KARMADA_KEY}/g" "${TEMP_PATH}"/secret-tmp.yaml

    sed -i'' -e "s/{{server_key}}/${KARMADA_KEY}/g" "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml
    sed -i'' -e "s/{{server_certificate}}/${KARMADA_CRT}/g" "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml

    kubectl apply -f "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    kubectl apply -f "${TEMP_PATH}"/secret-tmp.yaml
    kubectl apply -f "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml
    rm -rf "${TEMP_PATH}"
}

# install Karmada's APIs
function installCRDs() {
    local crd_path=$1

    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

    kubectl kustomize "${crd_path}"/_crds | kubectl apply -f -
}

# Use x.x.x.6 IP address, which is the same CIDR with the node address of the Kind cluster,
# as the loadBalancer service address of component karmada-interpreter-webhook-example.
interpreter_webhook_example_service_external_ip_prefix=$(echo $(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}") | awk -F. '{printf "%s.%s.%s",$1,$2,$3}')
interpreter_webhook_example_service_external_ip_address=${interpreter_webhook_example_service_external_ip_prefix}.6

# generate cert
util::cmd_must_exist "openssl"
util::cmd_must_exist_cfssl ${CFSSL_VERSION}
# create CA signers
util::create_signing_certkey "" "${CERT_DIR}" server '"client auth","server auth"'
util::create_signing_certkey "" "${CERT_DIR}" front-proxy '"client auth","server auth"'
# signs a certificate
util::create_certkey "" "${CERT_DIR}" "server-ca" karmada system:admin kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1" "${interpreter_webhook_example_service_external_ip_address}"
util::create_certkey "" "${CERT_DIR}" "front-proxy-ca" front-proxy-client front-proxy-client kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"

# create namespace for control plane components
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

# create service account, cluster role for controller-manager
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

KARMADA_CRT=$(base64 "${CERT_DIR}/karmada.crt" | tr -d '\r\n')
KARMADA_KEY=$(base64 "${CERT_DIR}/karmada.key" | tr -d '\r\n')
FRONT_PROXY_CA_CRT=$(base64 "${CERT_DIR}/front-proxy-ca.crt" | tr -d '\r\n')
FRONT_PROXY_CLIENT_CRT=$(base64 "${CERT_DIR}/front-proxy-client.crt" | tr -d '\r\n')
FRONT_PROXY_CLIENT_KEY=$(base64 "${CERT_DIR}/front-proxy-client.key" | tr -d '\r\n')
generate_cert_secret

# deploy karmada etcd
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-etcd.yaml"

# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready "${ETCD_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

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
cp "${REPO_ROOT}"/artifacts/deploy/karmada-apiserver.yaml "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
sed -i'' -e "s/{{service_type}}/${KARMADA_APISERVER_SERVICE_TYPE}/g" "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
echo -e "\nApply dynamic rendered apiserver service in ${TEMP_PATH_APISERVER}/karmada-apiserver.yaml."
kubectl apply -f "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml

# Wait for karmada-apiserver to come up before launching the rest of the components.
util::wait_pod_ready "${APISERVER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# get Karmada apiserver IP at remote mode
if [ "${HOST_CLUSTER_TYPE}" = "remote" ]; then
  case $KARMADA_APISERVER_SERVICE_TYPE in
    ClusterIP)
      KARMADA_APISERVER_IP=$(kubectl get pod -l app=karmada-apiserver -n "${KARMADA_SYSTEM_NAMESPACE}" -o=jsonpath='{.items[0].status.podIP}')
    ;;
    LoadBalancer)
      if util::wait_service_external_ip "karmada-apiserver" "${KARMADA_SYSTEM_NAMESPACE}"; then
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
util::append_client_kubeconfig "${HOST_CLUSTER_KUBECONFIG}" "${CERT_DIR}/karmada.crt" "${CERT_DIR}/karmada.key" "${KARMADA_APISERVER_IP}" "${KARMADA_APISERVER_SECURE_PORT}" karmada-apiserver

# deploy kube controller manager
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/kube-controller-manager.yaml"
# deploy aggregated-apiserver on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-aggregated-apiserver.yaml"
util::wait_pod_ready "${KARMADA_AGGREGATION_APISERVER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
# deploy karmada-search on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-search.yaml"
util::wait_pod_ready "${KARMADA_SEARCH_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# install CRD APIs on karmada apiserver.
if ! kubectl config use-context karmada-apiserver > /dev/null 2>&1;
then
  echo -e "ERROR: failed to use context: karmada-apiserver not in ${HOST_CLUSTER_KUBECONFIG}."
  recover_kubeconfig
  exit 1
fi

TEMP_PATH_CRDS=$(mktemp -d)
cp -rf "${REPO_ROOT}"/charts/karmada/_crds "${TEMP_PATH_CRDS}"
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_resourcebindings.yaml"
util::fill_cabundle "${ROOT_CA_FILE}" "${TEMP_PATH_CRDS}/_crds/patches/webhook_in_clusterresourcebindings.yaml"
installCRDs "${TEMP_PATH_CRDS}"
rm -rf "${TEMP_PATH_CRDS}"

# deploy webhook configurations on karmada apiserver
util::deploy_webhook_configuration "${ROOT_CA_FILE}" "${REPO_ROOT}/artifacts/deploy/webhook-configuration.yaml"

# deploy APIService on karmada apiserver for karmada-aggregated-apiserver
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-aggregated-apiserver-apiservice.yaml"
# make sure apiservice for v1alpha1.cluster.karmada.io is Available
util::wait_apiservice_ready "${KARMADA_AGGREGATION_APISERVER_LABEL}"

# deploy APIService on karmada apiserver for karmada-search
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-search-apiservice.yaml"
# make sure apiservice for v1alpha1.search.karmada.io is Available
util::wait_apiservice_ready "${KARMADA_SEARCH_LABEL}"

# deploy cluster proxy rbac for admin
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/cluster-proxy-admin-rbac.yaml"

kubectl config use-context "${HOST_CLUSTER_NAME}"

# deploy controller-manager on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-controller-manager.yaml"
# deploy scheduler on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-scheduler.yaml"
# deploy descheduler on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-descheduler.yaml"
# deploy webhook on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-webhook.yaml"

# make sure all karmada control plane components are ready
util::wait_pod_ready "${KARMADA_CONTROLLER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KARMADA_SCHEDULER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KUBE_CONTROLLER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KARMADA_WEBHOOK_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
