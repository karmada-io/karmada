#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Use `hack/generate-proto.sh` to generate proto files.

export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

GO111MODULE=on go install golang.org/x/tools/cmd/goimports
GO111MODULE=on go install k8s.io/code-generator/cmd/go-to-protobuf
GO111MODULE=on go install github.com/gogo/protobuf/protoc-gen-gogo

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
  --proto-import ./vendor \
  --proto-import ./third_party/protobuf/

# The `go-to-protobuf` tool will modify all import proto files in vendor, so we should use go mod vendor to prevent.
go mod vendor

SERVICE_PROTO_FILES=$(find . -name "service.proto")

for file in ${SERVICE_PROTO_FILES[*]}; do
  protoc \
    --proto_path=. \
    --proto_path=./vendor \
    --gogo_out=plugins=grpc:. \
    $file
done
