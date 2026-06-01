#!/bin/bash
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


set -o errexit
set -o nounset
set -o pipefail

KARMADA_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

DEFAULT_GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export GOPATH=${DEFAULT_GOPATH}
export PATH=$PATH:$GOPATH/bin

PROTOC_VERSION="23.4"
PROTOC_TAG="v${PROTOC_VERSION}"
PROTOC_DIR="${KARMADA_ROOT}/_tmp/protoc"

GO111MODULE=on go install golang.org/x/tools/cmd/goimports
GO111MODULE=on go install k8s.io/code-generator/cmd/go-to-protobuf
GO111MODULE=on go install github.com/gogo/protobuf/protoc-gen-gogo
GO111MODULE=on go install github.com/vektra/mockery/v3

setup_protoc() {
  # If the pinned protoc already exists and matches, reuse it.
  if [[ -x "${PROTOC_DIR}/bin/protoc" ]]; then
    if [[ "$("${PROTOC_DIR}/bin/protoc" --version 2>/dev/null)" == "libprotoc ${PROTOC_VERSION}"* ]]; then
      export PATH="${PROTOC_DIR}/bin:${PATH}"
      return 0
    fi
    # Version mismatch; remove stale binary and re-download below.
    rm -rf "${PROTOC_DIR}"
  fi

  # If system protoc already matches, keep using it (no download needed).
  if command -v protoc >/dev/null 2>&1; then
    if [[ "$(protoc --version 2>/dev/null)" == "libprotoc ${PROTOC_VERSION}"* ]]; then
      return 0
    fi
  fi

  rm -rf "${PROTOC_DIR}"
  mkdir -p "${PROTOC_DIR}"

  local os arch zip_name url tmp_zip
  os="$(uname | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  # Map host OS/arch to the suffixes used by protobuf release assets.
  # Note: protobuf releases use "osx" (not "darwin") and "aarch_64" (not "aarch64").
  case "${os}" in
    linux) os="linux" ;;
    darwin) os="osx" ;;
    *) echo "Unsupported OS for protoc download: ${os}" >&2; exit 1 ;;
  esac

  case "${arch}" in
    x86_64|amd64) arch="x86_64" ;;
    aarch64|arm64) arch="aarch_64" ;;
    *) echo "Unsupported arch for protoc download: ${arch}" >&2; exit 1 ;;
  esac

  zip_name="protoc-${PROTOC_VERSION}-${os}-${arch}.zip"

  url="https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_TAG}/${zip_name}"
  tmp_zip="${KARMADA_ROOT}/_tmp/${zip_name}"

  if command -v curl >/dev/null 2>&1; then
    curl -sSLf -o "${tmp_zip}" "${url}"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "${tmp_zip}" "${url}"
  else
    echo "Neither curl nor wget is available to download protoc" >&2
    exit 1
  fi

  if ! command -v unzip >/dev/null 2>&1; then
    echo "unzip is required to install protoc ${PROTOC_VERSION}" >&2
    exit 1
  fi

  unzip -q "${tmp_zip}" -d "${PROTOC_DIR}"
  rm -f "${tmp_zip}"

  export PATH="${PROTOC_DIR}/bin:${PATH}"
}

# Make dummy GOPATH for go-to-protobuf to generate the files to repo root.
# It is useful for case that karmada repo not in the real GOPATH.
go_path="${KARMADA_ROOT}/_go"
cleanup() {
  rm -rf "${go_path}"
}
trap "cleanup" EXIT SIGINT

cleanup

setup_protoc

source "${KARMADA_ROOT}"/hack/util.sh
util:create_gopath_tree "${KARMADA_ROOT}" "${go_path}"
export GOPATH="${go_path}"

# https://github.com/kubernetes/kubernetes/blob/release-1.23/hack/update-generated-protobuf-dockerized.sh
if [[ -z "$(which protoc)" || $(protoc --version | sed -r "s/libprotoc ([0-9]+).*/\1/g") -lt 3 ]]; then
  echo "Generating protobuf requires protoc 3.0.0-beta1 or newer. Please download and"
  echo "install the platform appropriate Protobuf package for your OS: "
  echo
  echo "  https://github.com/protocolbuffers/protobuf/releases"
  echo
  echo "WARNING: Protobuf changes are not being validated"
  exit 1
fi

PACKAGES=(
  github.com/karmada-io/karmada/pkg/estimator/pb
)

APIMACHINERY_PKGS=(
  -k8s.io/apimachinery/pkg/util/intstr
  -k8s.io/apimachinery/pkg/api/resource
  -k8s.io/apimachinery/pkg/runtime/schema
  -k8s.io/apimachinery/pkg/runtime
  -k8s.io/apimachinery/pkg/apis/meta/v1
  -k8s.io/api/core/v1
)

go-to-protobuf \
  --go-header-file=./hack/boilerplate/boilerplate.go.txt \
  --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}") \
  --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
  --proto-import="${KARMADA_ROOT}/vendor" \
  --proto-import="${KARMADA_ROOT}/third_party/protobuf" \
  --output-dir="${GOPATH}/src"

go generate ./pkg/estimator/service
