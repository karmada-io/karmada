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
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}
PULL_MODE_CLUSTER_KUBECONFIG=${PULL_MODE_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/member3.config"}

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
    local karmada_ca=$(base64 "${ROOT_CA_FILE}" | tr -d '\r\n')

    local TEMP_PATH=$(mktemp -d)
    cp -rf ${REPO_ROOT}/artifacts/deploy/karmada-cert-secret.yaml ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    cp -rf ${REPO_ROOT}/artifacts/deploy/secret.yaml ${TEMP_PATH}/secret-tmp.yaml
    cp -rf ${REPO_ROOT}/artifacts/deploy/karmada-webhook-cert-secret.yaml ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_cer}}/${KARMADA_CRT}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_key}}/${KARMADA_KEY}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" ${TEMP_PATH}/secret-tmp.yaml
    sed -i "s/{{client_cer}}/${KARMADA_CRT}/g" ${TEMP_PATH}/secret-tmp.yaml
    sed -i "s/{{client_key}}/${KARMADA_KEY}/g" ${TEMP_PATH}/secret-tmp.yaml

    sed -i "s/{{server_key}}/${KARMADA_KEY}/g" ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml
    sed -i "s/{{server_certificate}}/${KARMADA_CRT}/g" ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml

    kubectl apply -f ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    kubectl apply -f ${TEMP_PATH}/secret-tmp.yaml
    kubectl apply -f ${TEMP_PATH}/karmada-webhook-cert-secret-tmp.yaml
    rm -rf "${TEMP_PATH}"
}

# installCRDs installs crd APIs of karmada in host cluster
function installCRDs() {
    if [[ ! -f ${KARMADA_APISERVER_CONFIG} ]]; then
        echo "Please provide kubeconfig to connect karmada apiserver"
        return 1
    fi

    # install APIs
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/cluster.karmada.io_clusters.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_propagationpolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_clusterpropagationpolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_overridepolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_clusteroverridepolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/work.karmada.io_works.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/work.karmada.io_resourcebindings.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/work.karmada.io_clusterresourcebindings.yaml"
}

# deploy karmada agent in pull mode member clusters
function deploy_karmada_agent() {
    export KUBECONFIG="${PULL_MODE_CLUSTER_KUBECONFIG}"

    # create namespace for karmada agent
    kubectl apply -f "${REPO_ROOT}/artifacts/agent/namespace.yaml"

    # create service account, cluster role for karmada agent
    kubectl apply -f "${REPO_ROOT}/artifacts/agent/serviceaccount.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/agent/clusterrole.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/agent/clusterrolebinding.yaml"

    # create secret
    kubectl create secret generic karmada-kubeconfig --from-file=karmada-kubeconfig="$KARMADA_APISERVER_CONFIG" -n "${KARMADA_SYSTEM_NAMESPACE}"

    # deploy karmada agent
    cp "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml.tmp
    sed -i "s/{{member_cluster_name}}/${PULL_MODE_CLUSTER_NAME}/g" "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml
    kubectl apply -f "${REPO_ROOT}/artifacts/agent/karmada-agent.yaml"
    mv "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml.tmp "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml

    # Wait for karmada-etcd to come up before launching the rest of the components.
    util::wait_pod_ready "${AGENT_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
    if [[ $? -ne 0 ]]; then
       echo "failed to deploy karmada agent in pull mode cluster"
       return 1
    fi
}

#step0: prepare
# install kind
util::install_tools sigs.k8s.io/kind v0.10.0

#step1. create host cluster and member clusters in parallel
util::create_cluster ${HOST_CLUSTER_NAME} ${HOST_CLUSTER_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}
util::create_cluster ${MEMBER_CLUSTER_1_NAME} ${MEMBER_CLUSTER_1_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}
util::create_cluster ${MEMBER_CLUSTER_2_NAME} ${MEMBER_CLUSTER_2_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}
util::create_cluster ${PULL_MODE_CLUSTER_NAME} ${PULL_MODE_CLUSTER_KUBECONFIG} ${CLUSTER_VERSION} ${KIND_LOG_FILE}

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
KARMADA_CRT=$(sudo base64 "${CERT_DIR}/karmada.crt" | tr -d '\r\n')
KARMADA_KEY=$(sudo base64 "${CERT_DIR}/karmada.key" | tr -d '\r\n')
util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${KARMADA_CRT}" "${KARMADA_KEY}" "${KARMADA_APISERVER_IP}" "${KARMADA_APISERVER_SECURE_PORT}" karmada-apiserver

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
util::wait_pod_ready ${ETCD_POD_LABEL} "${KARMADA_SYSTEM_NAMESPACE}"

# deploy karmada apiserver
TEMP_PATH=$(mktemp -d)
cp -rf ${REPO_ROOT}/artifacts/deploy/karmada-apiserver.yaml ${TEMP_PATH}/karmada-apiserver-tmp.yaml
sed -i "s/{{api_addr}}/${KARMADA_APISERVER_IP}/g" ${TEMP_PATH}/karmada-apiserver-tmp.yaml
kubectl apply -f "${TEMP_PATH}/karmada-apiserver-tmp.yaml"
rm -rf "${TEMP_PATH}"

# wait for karmada-apiserver to come up before launching the rest of the components.
util::wait_pod_ready ${APISERVER_POD_LABEL} "${KARMADA_SYSTEM_NAMESPACE}"

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
util::wait_pod_ready "${KARMADA_CONTROLLER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KARMADA_SCHEDULER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KUBE_CONTROLLER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KARMADA_WEBHOOK_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# wait until the member cluster ready
util::check_clusters_ready "${MEMBER_CLUSTER_1_KUBECONFIG}" "$MEMBER_CLUSTER_1_NAME"
util::check_clusters_ready "${MEMBER_CLUSTER_2_KUBECONFIG}" "$MEMBER_CLUSTER_2_NAME"

#step8. join push mode member clusters
export KUBECONFIG=${KARMADA_APISERVER_CONFIG}
${KARMADACTL_BIN} join member1 --cluster-kubeconfig="${MEMBER_CLUSTER_1_KUBECONFIG}"
${KARMADACTL_BIN} join member2 --cluster-kubeconfig="${MEMBER_CLUSTER_2_KUBECONFIG}"

# wait until the pull mode cluster ready
util::check_clusters_ready ${PULL_MODE_CLUSTER_KUBECONFIG} "$PULL_MODE_CLUSTER_NAME"
kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="$PULL_MODE_CLUSTER_NAME"

#step9. deploy karmada agent in pull mode member clusters
deploy_karmada_agent

function print_success() {
  echo
  echo "Local Karmada is running."
  echo "To start using your karmada, run:"
cat <<EOF
  export KUBECONFIG=$KARMADA_APISERVER_CONFIG
EOF
}

print_success
