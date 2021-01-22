#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/karmada-io/karmada/pkg/generated github.com/karmada-io/karmada/pkg/apis \
  "propagationstrategy:v1alpha1 membercluster:v1alpha1" \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt

echo "Generating with register-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1
