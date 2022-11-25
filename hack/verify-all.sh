#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# Show progress
set -x

# Orders are determined by two factors:
# (1) Less Execution time item should be executed first.
# (2) More likely to fail item should be executed first.
bash "$REPO_ROOT/hack/verify-lifted.sh"
bash "$REPO_ROOT/hack/verify-import-aliases.sh"
bash "$REPO_ROOT/hack/verify-staticcheck.sh"
bash "$REPO_ROOT/hack/verify-mocks.sh"
bash "$REPO_ROOT/hack/verify-gofmt.sh"
bash "$REPO_ROOT/hack/verify-vendor.sh"
bash "$REPO_ROOT/hack/verify-swagger-docs.sh"
bash "$REPO_ROOT/hack/verify-crdgen.sh"
bash "$REPO_ROOT/hack/verify-codegen.sh"
