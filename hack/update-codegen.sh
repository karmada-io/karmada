#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/conversion-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/lister-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/informer-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/openapi-gen
export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

go_path="${REPO_ROOT}/_go"
cleanup() {
  rm -rf "${go_path}"
}
trap "cleanup" EXIT SIGINT

cleanup

source "${REPO_ROOT}"/hack/util.sh
util:create_gopath_tree "${REPO_ROOT}" "${go_path}"
export GOPATH="${go_path}"

echo "Generating with deepcopy-gen"
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster \
  --output-package=github.com/karmada-io/karmada/pkg/apis/cluster \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/search \
  --output-package=github.com/karmada-io/karmada/pkg/apis/search \
  --output-file-base=zz_generated.deepcopy
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-file-base=zz_generated.deepcopy

echo "Generating with register-gen"
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-file-base=zz_generated.register
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-file-base=zz_generated.register

echo "Generating with conversion-gen"
conversion-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-file-base=zz_generated.conversion
conversion-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-file-base=zz_generated.conversion

echo "Generating with client-gen"
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1,github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1,github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/generated/clientset \
  --clientset-name=versioned
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/operator/pkg/generated/clientset \
  --clientset-name=versioned

echo "Generating with lister-gen"
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1,github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1,github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/generated/listers
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/operator/pkg/generated/listers

echo "Generating with informer-gen"
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1,github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1,github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  --versioned-clientset-package=github.com/karmada-io/karmada/pkg/generated/clientset/versioned \
  --listers-package=github.com/karmada-io/karmada/pkg/generated/listers \
  --output-package=github.com/karmada-io/karmada/pkg/generated/informers
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --versioned-clientset-package=github.com/karmada-io/karmada/operator/pkg/generated/clientset/versioned \
  --listers-package=github.com/karmada-io/karmada/operator/pkg/generated/listers \
  --output-package=github.com/karmada-io/karmada/operator/pkg/generated/informers

echo "Generating with openapi-gen"
openapi-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1" \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1" \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1" \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2" \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1" \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1" \
  --input-dirs "k8s.io/api/core/v1,k8s.io/apimachinery/pkg/api/resource" \
  --input-dirs "k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/version" \
  --input-dirs "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1,k8s.io/api/admissionregistration/v1,k8s.io/api/networking/v1" \
  --input-dirs "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1" \
  --output-package "github.com/karmada-io/karmada/pkg/generated/openapi" \
  -O zz_generated.openapi

