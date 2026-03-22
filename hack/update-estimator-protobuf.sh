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

GO111MODULE=on go install golang.org/x/tools/cmd/goimports
GO111MODULE=on go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
GO111MODULE=on go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
GO111MODULE=on go install github.com/vektra/mockery/v3

# Make dummy GOPATH for go-to-protobuf to generate the files to repo root.
# It is useful for case that karmada repo not in the real GOPATH.
go_path="${KARMADA_ROOT}/_go"
cleanup() {
  rm -rf "${go_path}"
}
trap "cleanup" EXIT SIGINT

cleanup

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

protoc --go_out=. --go_opt=paths=source_relative \
  -I . -I "${KARMADA_ROOT}/vendor" \
  pkg/estimator/pb/generated.proto

go generate ./pkg/estimator/service
