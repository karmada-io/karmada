#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

echo "Generating with deepcopy-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen
export GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
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
  --input-dirs=github.com/karmada-io/karmada/examples/customresourceexplorer/apis/workload/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/examples/customresourceexplorer/apis/workload/v1alpha1 \
  --output-file-base=zz_generated.deepcopy

echo "Generating with register-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
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
  --input-dirs=github.com/karmada-io/karmada/examples/customresourceexplorer/apis/workload/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/examples/customresourceexplorer/apis/workload/v1alpha1 \
  --output-file-base=zz_generated.register

echo "Generating with client-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/generated/clientset \
  --clientset-name=versioned

echo "Generating with lister-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/lister-gen
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --output-package=github.com/karmada-io/karmada/pkg/generated/listers

echo "Generating with informer-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/informer-gen
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-dirs=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  --versioned-clientset-package=github.com/karmada-io/karmada/pkg/generated/clientset/versioned \
  --listers-package=github.com/karmada-io/karmada/pkg/generated/listers \
  --output-package=github.com/karmada-io/karmada/pkg/generated/informers

