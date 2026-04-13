#!/usr/bin/env bash
set -euo pipefail

KUBECONFIG_PATH="${KUBECONFIG_PATH:-$HOME/.kube/karmada.config}"
USER_NAME="${USER_NAME:-karmada-apiserver}"
CLUSTER_NAME="${CLUSTER_NAME:-karmada-apiserver}"
NS="${NS:-monitoring}"

CLIENT_TLS_SECRET="${CLIENT_TLS_SECRET:-karmada-apiserver-client-tls}"
SERVER_CA_SECRET="${SERVER_CA_SECRET:-karmada-apiserver-server-ca}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: missing $1" >&2; exit 1; }; }
need kubectl
need openssl
need base64
need sed
need grep

b64dec() { base64 -d 2>/dev/null || base64 -D; }

CRT_B64="$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --raw -o jsonpath="{.users[?(@.name=='$USER_NAME')].user.client-certificate-data}")"
KEY_B64="$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --raw -o jsonpath="{.users[?(@.name=='$USER_NAME')].user.client-key-data}")"
CA_B64="$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --raw -o jsonpath="{.clusters[?(@.name=='$CLUSTER_NAME')].cluster.certificate-authority-data}")"

if [[ -z "$CRT_B64" ]]; then
  CRT_PATH="$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --raw -o jsonpath="{.users[?(@.name=='$USER_NAME')].user.client-certificate}")"
  [[ -n "$CRT_PATH" ]] || { echo "ERROR: no client-certificate-data or client-certificate path for user=$USER_NAME" >&2; exit 1; }
fi
if [[ -z "$KEY_B64" ]]; then
  KEY_PATH="$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --raw -o jsonpath="{.users[?(@.name=='$USER_NAME')].user.client-key}")"
  [[ -n "$KEY_PATH" ]] || { echo "ERROR: no client-key-data or client-key path for user=$USER_NAME" >&2; exit 1; }
fi
if [[ -z "$CA_B64" ]]; then
  CA_PATH="$(kubectl --kubeconfig "$KUBECONFIG_PATH" config view --raw -o jsonpath="{.clusters[?(@.name=='$CLUSTER_NAME')].cluster.certificate-authority}")"
  [[ -n "$CA_PATH" ]] || { echo "ERROR: no certificate-authority-data or certificate-authority path for cluster=$CLUSTER_NAME" >&2; exit 1; }
fi

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

if [[ -n "${CRT_B64:-}" ]]; then
  printf '%s' "$CRT_B64" | b64dec > "$TMP/tls.crt"
else
  cp "$CRT_PATH" "$TMP/tls.crt"
fi

if [[ -n "${KEY_B64:-}" ]]; then
  printf '%s' "$KEY_B64" | b64dec > "$TMP/tls.key"
else
  cp "$KEY_PATH" "$TMP/tls.key"
fi

if [[ -n "${CA_B64:-}" ]]; then
  printf '%s' "$CA_B64" | b64dec > "$TMP/ca.crt"
else
  cp "$CA_PATH" "$TMP/ca.crt"
fi


echo "Validating TLS materials..."
openssl x509 -in "$TMP/tls.crt" -noout >/dev/null || { echo "ERROR: tls.crt is not a valid X.509 cert" >&2; exit 1; }

openssl pkey -in "$TMP/tls.key" -noout >/dev/null || { echo "ERROR: tls.key is not a valid private key" >&2; exit 1; }
openssl x509 -in "$TMP/ca.crt" -noout >/dev/null   || { echo "ERROR: ca.crt is not a valid X.509 cert" >&2; exit 1; }

echo "Client cert subject/issuer:"
openssl x509 -in "$TMP/tls.crt" -noout -subject -issuer | sed 's/^/  /'

ONE_SECRET_NAME="${ONE_SECRET_NAME:-karmada-apiserver-scrape-tls}"

kubectl --kubeconfig ~/.kube/karmada.config --context karmada-host -n "$NS" \
  create secret generic "$ONE_SECRET_NAME" \
  --from-file=tls.crt="$TMP/tls.crt" \
  --from-file=tls.key="$TMP/tls.key" \
  --from-file=ca.crt="$TMP/ca.crt" \
  --dry-run=client -o yaml \
| kubectl --kubeconfig ~/.kube/karmada.config --context karmada-host apply -f -

echo "OK."