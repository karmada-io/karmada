#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# vendor should be updated first because we build code-gen tools from vendor.
bash "$REPO_ROOT/hack/update-vendor.sh"
bash "$REPO_ROOT/hack/update-codegen.sh"
bash "$REPO_ROOT/hack/update-crdgen.sh"
bash "$REPO_ROOT/hack/update-estimator-protobuf.sh"
bash "$REPO_ROOT/hack/update-import-aliases.sh"
bash "$REPO_ROOT/hack/update-swagger-docs.sh"
bash "$REPO_ROOT/hack/update-lifted.sh"
bash "$REPO_ROOT/hack/update-mocks.sh"
bash "$REPO_ROOT/hack/update-gofmt.sh"
