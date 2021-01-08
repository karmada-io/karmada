#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# The host cluster name which used to install karmada control plane components.
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
API_SECURE_PORT=${API_SECURE_PORT:-5443}

CERT_DIR=${CERT_DIR:-"/var/run/karmada"}
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
CFSSL_VERSION="v1.5.0"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
ROOT_CA_FILE=${CERT_DIR}/server-ca.crt
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

source hack/util.sh

# check whether openssl is installed.
function ensure_openssl {
    OPENSSL_BIN=$(command -v openssl)
    if [[ ! -x ${OPENSSL_BIN} ]]; then
      echo "Please install openssl and verify they are in \$PATH."
      exit 1
    fi
}

# downloads cfssl/cfssljson if they do not already exist in PATH
function ensure-cfssl {
    if command -v cfssl &>/dev/null && command -v cfssljson &>/dev/null; then
        CFSSL_BIN=$(command -v cfssl)
        CFSSLJSON_BIN=$(command -v cfssljson)
        return 0
    fi

    util::install_tools "github.com/cloudflare/cfssl/cmd/..." ${CFSSL_VERSION}

    GOPATH=$(go env | grep GOPATH | awk -F '=' '{print $2}'| sed 's/\"//g')
    CFSSL_BIN="${GOPATH}/bin/cfssl"
    CFSSLJSON_BIN="${GOPATH}/bin/cfssljson"
    if [[ ! -x ${CFSSL_BIN} || ! -x ${CFSSLJSON_BIN} ]]; then
      echo "Failed to download 'cfssl'. Please install cfssl and cfssljson and verify they are in \$PATH."
      echo "Hint: export PATH=\$PATH:\$GOPATH/bin; go get -u github.com/cloudflare/cfssl/cmd/..."
      exit 1
    fi
}

# creates a client CA, args are sudo, dest-dir, ca-id, purpose
function create_signing_certkey {
    local sudo=$1
    local dest_dir=$2
    local id=$3
    local purpose=$4
    # Create client ca
    ${sudo} /usr/bin/env bash -e <<EOF
    rm -f "${dest_dir}/${id}-ca.crt" "${dest_dir}/${id}-ca.key"
    ${OPENSSL_BIN} req -x509 -sha256 -new -nodes -days 365 -newkey rsa:2048 -keyout "${dest_dir}/${id}-ca.key" -out "${dest_dir}/${id}-ca.crt" -subj "/C=xx/ST=x/L=x/O=x/OU=x/CN=ca/emailAddress=x/"
    echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment",${purpose}]}}}' > "${dest_dir}/${id}-ca-config.json"
EOF
}

# signs a certificate: args are sudo, dest-dir, ca, filename (roughly), subject, hosts...
function create_certkey {
    local sudo=$1
    local dest_dir=$2
    local ca=$3
    local id=$4
    local cn=${5:-$4}
    local hosts=""
    local SEP=""
    shift 5
    while [ -n "${1:-}" ]; do
        hosts+="${SEP}\"$1\""
        SEP=","
        shift 1
    done
    ${sudo} /usr/bin/env bash -e <<EOF
    cd ${dest_dir}
    echo '{"CN":"${cn}","hosts":[${hosts}],"names":[{"O":"system:masters"}],"key":{"algo":"rsa","size":2048}}' | ${CFSSL_BIN} gencert -ca=${ca}.crt -ca-key=${ca}.key -config=${ca}-config.json - | ${CFSSLJSON_BIN} -bare ${id}
    mv "${id}-key.pem" "${id}.key"
    mv "${id}.pem" "${id}.crt"
    rm -f "${id}.csr"
EOF
}

# create CA signers and certs
function generate_certs {
    # create CA signers
    create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"client auth","server auth"'

    # signs a certificate
    create_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" karmada system:admin kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "localhost" "127.0.0.1"
}

# generate kubeconfig file for kubectl
function generate_kubeconfig {
    KARMADA_API_IP=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${HOST_CLUSTER_NAME}-control-plane")
    write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${KARMADA_API_IP}" "${API_SECURE_PORT}" karmada-apiserver
}

# creates a self-contained kubeconfig: args are sudo, dest-dir, ca file, host, port, client id, token(optional)
function write_client_kubeconfig {
    local sudo=$1
    local dest_dir=$2
    local ca_file=$3
    local api_host=$4
    local api_port=$5
    local client_id=$6
    local token=${7:-}
    cat <<EOF | ${sudo} tee "${dest_dir}"/"${client_id}".config > /dev/null
apiVersion: v1
kind: Config
clusters:
  - cluster:
      "insecure-skip-tls-verify": true
      server: https://${api_host}:${api_port}/
    name: karmada-apiserver
users:
  - user:
      token: ${token}
      client-certificate: ${dest_dir}/karmada.crt
      client-key: ${dest_dir}/karmada.key
    name: karmada-apiserver
contexts:
  - context:
      cluster: karmada-apiserver
      user: karmada-apiserver
    name: karmada-apiserver
current-context: karmada-apiserver
EOF
}

# generate a secret to store the certificates
function generate_cert_secret {
    local karmada_crt_file=${CERT_DIR}/karmada.crt
    local karmada_key_file=${CERT_DIR}/karmada.key

    sudo chmod 0644 ${karmada_crt_file}
    sudo chmod 0644 ${karmada_key_file}

    local karmada_ca=$(sudo cat ${ROOT_CA_FILE} | base64 | tr "\n" " "|sed s/[[:space:]]//g)
    local karmada_crt=$(sudo cat ${karmada_crt_file} | base64 | tr "\n" " "|sed s/[[:space:]]//g)
    local karmada_key=$(sudo cat ${karmada_key_file} | base64 | tr "\n" " "|sed s/[[:space:]]//g)

    TEMP_PATH=$(mktemp -d)
    cp -rf ${SCRIPT_ROOT}/artifacts/deploy/karmada-cert-secret.yaml ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    cp -rf ${SCRIPT_ROOT}/artifacts/deploy/secret.yaml ${TEMP_PATH}/secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_cer}}/${karmada_crt}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_key}}/${karmada_key}/g" ${TEMP_PATH}/karmada-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" ${TEMP_PATH}/secret-tmp.yaml
    sed -i "s/{{client_cer}}/${karmada_crt}/g" ${TEMP_PATH}/secret-tmp.yaml
    sed -i "s/{{client_key}}/${karmada_key}/g" ${TEMP_PATH}/secret-tmp.yaml

    kubectl apply -f ${TEMP_PATH}/karmada-cert-secret-tmp.yaml
    kubectl apply -f ${TEMP_PATH}/secret-tmp.yaml
    rm -rf "${TEMP_PATH}"
}

ensure_openssl
ensure-cfssl
generate_certs
generate_kubeconfig
generate_cert_secret
