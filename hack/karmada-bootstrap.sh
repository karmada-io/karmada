#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# This script starts a local karmada control plane based on current codebase and with a certain number of clusters joined.	REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# This script intended used in following scenarios:	source ${REPO_ROOT}/hack/util.sh
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to setup test environment automatically.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source ${REPO_ROOT}/hack/util.sh

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
HOST_CLUSTER_KUBECONFIG=${HOST_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada-host.config"}
MEMBER_CLUSTER_1_KUBECONFIG=${MEMBER_CLUSTER_1_KUBECONFIG:-"${KUBECONFIG_PATH}/member1.config"}
MEMBER_CLUSTER_2_KUBECONFIG=${MEMBER_CLUSTER_2_KUBECONFIG:-"${KUBECONFIG_PATH}/member2.config"}
CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.19.1"}

CERT_DIR=${CERT_DIR:-"/var/run/karmada"}
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
KARMADA_APISERVER_CONFIG="${CERT_DIR}/karmada-apiserver.config"
KARMADA_APISERVER_SECURE_PORT=${KARMADA_APISERVER_SECURE_PORT:-5443}

CFSSL_VERSION="v1.5.0"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
ROOT_CA_FILE=${CERT_DIR}/server-ca.crt

KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}

# generate a secret to store the certificates
function generate_cert_secret {
    local karmada_crt_file=${CERT_DIR}/karmada.crt
    local karmada_key_file=${CERT_DIR}/karmada.key

    sudo chmod 0644 ${karmada_crt_file}
    sudo chmod 0644 ${karmada_key_file}

    local karmada_ca=$(sudo cat ${ROOT_CA_FILE} | base64 | tr "\n" " "|sed s/[[:space:]]//g)
    local karmada_crt=$(sudo cat ${karmada_crt_file} | base64 | tr "\n" " "|sed s/[[:space:]]//g)
    local karmada_key=$(sudo cat ${karmada_key_file} | base64 | tr "\n" " "|sed s/[[:space:]]//g)

    local TEMP_PATH=$(mktemp -d)
    cp -rf ${REPO_ROOT}/artifacts/deploy/karmada-cert-secret.yaml ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    cp -rf ${REPO_ROOT}/artifacts/deploy/secret.yaml ${TEMP_PATH}/secret-tmp.yaml
    cp -rf ${REPO_ROOT}/artifacts/deploy/karmada-webhook-cert-secret.yaml ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_cer}}/${karmada_crt}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_key}}/${karmada_key}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" ${TEMP_PATH}/secret-tmp.yaml
    sed -i "s/{{client_cer}}/${karmada_crt}/g" ${TEMP_PATH}/secret-tmp.yaml
    sed -i "s/{{client_key}}/${karmada_key}/g" ${TEMP_PATH}/secret-tmp.yaml

    sed -i "s/{{server_key}}/${karmada_key}/g" ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml
    sed -i "s/{{server_certificate}}/${karmada_crt}/g" ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml

    kubectl apply -f ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    kubectl apply -f ${TEMP_PATH}/secret-tmp.yaml
    kubectl apply -f ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml
    rm -rf "${TEMP_PATH}"
}

function installCRDs() {
    if [[ ! -f ${KARMADA_APISERVER_CONFIG} ]]; then
        echo "Please provide kubeconfig to connect karmada apiserver"
        return 1
    fi

    # install APIs
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/cluster.karmada.io_clusters.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_propagationpolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_propagationbindings.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_works.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_overridepolicies.yaml"
}


#step1. create host cluster and member clusters in parallel
util::create_cluster ${HOST_CLUSTER_NAME} ${HOST_CLUSTER_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}
util::create_cluster ${MEMBER_CLUSTER_1_NAME} ${MEMBER_CLUSTER_1_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}
util::create_cluster ${MEMBER_CLUSTER_2_NAME} ${MEMBER_CLUSTER_2_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}

#step2. make images and get karmadactl
export VERSION="latest"
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
make images

GO111MODULE=on go install "github.com/karmada-io/karmada/cmd/karmadactl"
GOPATH=$(go env | grep GOPATH | awk -F '=' '{print $2}'| sed 's/\"//g')
KARMADACTL_BIN="${GOPATH}/bin/karmadactl"

#step3. generate cert
util::cmd_must_exist "openssl"
util::cmd_must_exist_cfssl ${CFSSL_VERSION}
# create CA signers
util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"client auth","server auth"'
# signs a certificate
util::create_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" karmada system:admin kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"

#step4. wait until the host cluster ready
echo "Waiting for the host clusters to be ready..."
util::check_clusters_ready ${HOST_CLUSTER_KUBECONFIG} ${HOST_CLUSTER_NAME}

#step5. load components images to kind cluster
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"

#step6. generate kubeconfig and cert secret
KARMADA_APISERVER_IP=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${HOST_CLUSTER_NAME}-control-plane")
util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${KARMADA_APISERVER_IP}" "${KARMADA_APISERVER_SECURE_PORT}" karmada-apiserver

#step7. install karmada control plane components
export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
# create namespace for control plane components
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"
# create service account, cluster role for controller-manager
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

generate_cert_secret

# deploy karmada etcd
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-etcd.yaml"
# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready ${ETCD_POD_LABEL} "karmada-system"

# deploy karmada apiserver
TEMP_PATH=$(mktemp -d)
cp -rf ${REPO_ROOT}/artifacts/deploy/karmada-apiserver.yaml ${TEMP_PATH}/karmada-apiserver-tmp.yaml
sed -i "s/{{api_addr}}/${KARMADA_APISERVER_IP}/g" ${TEMP_PATH}/karmada-apiserver-tmp.yaml
kubectl apply -f "${TEMP_PATH}/karmada-apiserver-tmp.yaml"
rm -rf "${TEMP_PATH}"

# wait for karmada-apiserver to come up before launching the rest of the components.
util::wait_pod_ready ${APISERVER_POD_LABEL} "karmada-system"

# deploy kube controller manager
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/kube-controller-manager.yaml"

# install CRD APIs on karmada apiserver.
export KUBECONFIG=${KARMADA_APISERVER_CONFIG}
installCRDs

# deploy webhook configurations on karmada apiserver
util::deploy_webhook_configuration ${ROOT_CA_FILE} "${REPO_ROOT}/artifacts/deploy/webhook-configuration.yaml"

# deploy karmada-controller-manager on host cluster
export KUBECONFIG=${HOST_CLUSTER_KUBECONFIG}
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/controller-manager.yaml"
# deploy karmada-scheduler on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-scheduler.yaml"
# deploy karmada-webhook on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-webhook.yaml"

# make sure all karmada control plane components are ready
util::wait_pod_ready ${KARMADA_CONTROLLER_LABEL} "karmada-system"
util::wait_pod_ready ${KARMADA_SCHEDULER_LABEL} "karmada-system"
util::wait_pod_ready ${KUBE_CONTROLLER_POD_LABEL} "karmada-system"
util::wait_pod_ready ${KARMADA_WEBHOOK_LABEL} "karmada-system"

# wait until the member cluster ready
util::check_clusters_ready ${MEMBER_CLUSTER_1_KUBECONFIG} "member1"
util::check_clusters_ready ${MEMBER_CLUSTER_2_KUBECONFIG} "member2"

# join member clusters
export KUBECONFIG=${KARMADA_APISERVER_CONFIG}
${KARMADACTL_BIN} join member1 --cluster-kubeconfig="${MEMBER_CLUSTER_1_KUBECONFIG}"
${KARMADACTL_BIN} join member2 --cluster-kubeconfig="${MEMBER_CLUSTER_2_KUBECONFIG}"

function print_success() {
  echo
  echo "Local Karmada is running."
  echo "To start using your karmada, run:"
cat <<EOF
  export KUBECONFIG=${KUBECONFIG}
EOF
}

print_success
