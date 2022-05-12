#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

bash "$REPO_ROOT/hack/update-codegen.sh"
bash "$REPO_ROOT/hack/update-crdgen.sh"
bash "$REPO_ROOT/hack/update-estimator-protobuf.sh"
bash "$REPO_ROOT/hack/update-import-aliases.sh"
bash "$REPO_ROOT/hack/update-vendor.sh"
bash "$REPO_ROOT/hack/update-swagger-docs.sh"
bash "$REPO_ROOT/hack/update-lifted.sh"
