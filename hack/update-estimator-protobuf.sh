#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KARMADA_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

# Use `hack/generate-proto.sh` to generate proto files.

export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

GO111MODULE=on go install golang.org/x/tools/cmd/goimports
GO111MODULE=on go install k8s.io/code-generator/cmd/go-to-protobuf
GO111MODULE=on go install github.com/gogo/protobuf/protoc-gen-gogo
GO111MODULE=on go install github.com/vektra/mockery/v2

#ref https://github.com/kubernetes/kubernetes/blob/master/hack/update-generated-protobuf-dockerized.sh
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
  +k8s.io/apimachinery/pkg/util/intstr
  +k8s.io/apimachinery/pkg/api/resource
  +k8s.io/apimachinery/pkg/runtime/schema
  +k8s.io/apimachinery/pkg/runtime
  k8s.io/apimachinery/pkg/apis/meta/v1
  k8s.io/api/core/v1
)

${GOPATH}/bin/go-to-protobuf \
  --go-header-file=./hack/boilerplate/boilerplate.go.txt \
  --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}") \
  --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
  --proto-import="${KARMADA_ROOT}/vendor" \
  --proto-import="${KARMADA_ROOT}/third_party/protobuf"

go generate ./pkg/estimator/service

# The `go-to-protobuf` tool will modify all import proto files in vendor, so we should use go mod vendor to prevent.
go mod vendor
