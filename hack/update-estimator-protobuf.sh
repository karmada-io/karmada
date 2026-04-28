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

#
# Pin protoc to keep generated headers stable across environments.
# This repo's generated estimator protos should be generated with the pinned
# protoc version to avoid environment-dependent `// versions:` headers.
#
PROTOC_VERSION="23.4"
PROTOC_TAG="v${PROTOC_VERSION}"
PROTOC_DIR="${KARMADA_ROOT}/_tmp/protoc"

DEFAULT_GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export GOPATH=${DEFAULT_GOPATH}
export PATH=$PATH:$GOPATH/bin

GO111MODULE=on go install golang.org/x/tools/cmd/goimports
GO111MODULE=on go install google.golang.org/protobuf/cmd/protoc-gen-go
GO111MODULE=on go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
GO111MODULE=on go install github.com/vektra/mockery/v3

setup_protoc() {
  # If the pinned protoc already exists and matches, reuse it.
  if [[ -x "${PROTOC_DIR}/bin/protoc" ]]; then
    if [[ "$("${PROTOC_DIR}/bin/protoc" --version 2>/dev/null)" == libprotoc\ ${PROTOC_VERSION}* ]]; then
      export PATH="${PROTOC_DIR}/bin:${PATH}"
      return 0
    fi
    # Version mismatch; remove stale binary and re-download below.
    rm -rf "${PROTOC_DIR}"
  fi

  # If system protoc already matches, keep using it (no download needed).
  if command -v protoc >/dev/null 2>&1; then
    if [[ "$(protoc --version 2>/dev/null)" == libprotoc\ ${PROTOC_VERSION}* ]]; then
      return 0
    fi
  fi

  rm -rf "${PROTOC_DIR}"
  mkdir -p "${PROTOC_DIR}"

  local os arch platform zip_name url tmp_zip
  os="$(uname | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "${os}" in
    linux) os="linux" ;;
    darwin) os="darwin" ;;
    *) echo "Unsupported OS for protoc download: ${os}" >&2; exit 1 ;;
  esac

  case "${arch}" in
    x86_64|amd64) platform="${os}-x86_64" ;;
    aarch64|arm64) platform="${os}-aarch64" ;;
    *) echo "Unsupported arch for protoc download: ${arch}" >&2; exit 1 ;;
  esac

  # Protobuf release artifact names differ slightly for macOS.
  if [[ "${os}" == "darwin" ]]; then
    if [[ "${platform}" == "darwin-x86_64" ]]; then
      zip_name="protoc-${PROTOC_VERSION}-osx-x86_64.zip"
    elif [[ "${platform}" == "darwin-aarch64" ]]; then
      zip_name="protoc-${PROTOC_VERSION}-osx-aarch_64.zip"
    else
      echo "Unsupported darwin platform: ${platform}" >&2; exit 1
    fi
  else
    if [[ "${platform}" == "linux-x86_64" ]]; then
      zip_name="protoc-${PROTOC_VERSION}-linux-x86_64.zip"
    elif [[ "${platform}" == "linux-aarch64" ]]; then
      zip_name="protoc-${PROTOC_VERSION}-linux-aarch_64.zip"
    else
      echo "Unsupported linux platform: ${platform}" >&2; exit 1
    fi
  fi

  url="https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_TAG}/${zip_name}"
  tmp_zip="${KARMADA_ROOT}/_tmp/${zip_name}"

  if command -v curl >/dev/null 2>&1; then
    curl -sSL -o "${tmp_zip}" "${url}"
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
if [[ -z "$(command -v protoc)" || "$(protoc --version 2>/dev/null)" != libprotoc\ ${PROTOC_VERSION}* ]]; then
  echo "Generating protobuf requires protoc libprotoc ${PROTOC_VERSION}*." >&2
  exit 1
fi

protoc --go_out=. --go_opt=paths=source_relative \
  -I . -I "${KARMADA_ROOT}/vendor" \
  pkg/estimator/pb/estimator.proto

go generate ./pkg/estimator/service
