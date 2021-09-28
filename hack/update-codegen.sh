#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

# All API group names in the pkg/apis directory that need code generation
PKGS=(cluster/v1alpha1 policy/v1alpha1 work/v1alpha1 work/v1alpha2)

CLIENT_PATH=github.com/karmada-io/karmada/pkg
CLIENT_APIS=${CLIENT_PATH}/apis

ALL_PKGS=""
for path in "${PKGS[@]}"
do
  ALL_PKGS=$ALL_PKGS" $CLIENT_APIS/$path"
done

function join {
  local IFS="$1"
  shift
  result="$*"
}

join "," $ALL_PKGS
FULL_PKGS=$result

echo "Generating for API group:" "${PKGS[@]}"

echo "Generating with deepcopy-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen
export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="${FULL_PKGS}" \
  --output-file-base=zz_generated.deepcopy

echo "Generating with register-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="${FULL_PKGS}" \
  --output-file-base=zz_generated.register

echo "Generating with client-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input="${FULL_PKGS}" \
  --output-package="${CLIENT_PATH}/generated/clientset" \
  --clientset-name=versioned

echo "Generating with lister-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/lister-gen
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="${FULL_PKGS}" \
  --output-package="${CLIENT_PATH}/generated/listers"

echo "Generating with informer-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/informer-gen
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs="${FULL_PKGS}" \
  --versioned-clientset-package="${CLIENT_PATH}/generated/clientset/versioned" \
  --listers-package="${CLIENT_PATH}/generated/listers" \
  --output-package="${CLIENT_PATH}/generated/informers"
