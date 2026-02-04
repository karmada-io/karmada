#!/usr/bin/env bash
# Copyright 2020 The Karmada Authors.
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

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/conversion-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/client-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/lister-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/informer-gen
GO111MODULE=on go install k8s.io/kube-openapi/cmd/openapi-gen
GO111MODULE=on go install k8s.io/code-generator/cmd/applyconfiguration-gen
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
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/cluster
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha2
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/config/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/search/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/search
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1
deepcopy-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.deepcopy.go \
  github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1

echo "Generating with register-gen"
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha2
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/config/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/search/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1

echo "Generating with conversion-gen"
conversion-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.conversion.go \
  github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1
conversion-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.conversion.go \
  github.com/karmada-io/karmada/pkg/apis/search/v1alpha1

echo "Generating with openapi-gen"
openapi-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-pkg "github.com/karmada-io/karmada/pkg/generated/openapi" \
  --output-dir pkg/generated/openapi \
  --output-file zz_generated.openapi.go \
  "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2" \
  "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1" \
  "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1" \
  "k8s.io/api/core/v1" \
  "k8s.io/apimachinery/pkg/api/resource" \
  "k8s.io/apimachinery/pkg/apis/meta/v1" \
  "k8s.io/apimachinery/pkg/runtime" \
  "k8s.io/apimachinery/pkg/version" \
  "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1" \
  "k8s.io/api/admissionregistration/v1" \
  "k8s.io/api/networking/v1" \
  "k8s.io/metrics/pkg/apis/custom_metrics" \
  "k8s.io/metrics/pkg/apis/custom_metrics/v1beta1" \
  "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2" \
  "k8s.io/metrics/pkg/apis/external_metrics" \
  "k8s.io/metrics/pkg/apis/external_metrics/v1beta1" \
  "k8s.io/metrics/pkg/apis/metrics" \
  "k8s.io/metrics/pkg/apis/metrics/v1beta1" \
  "k8s.io/api/autoscaling/v2"
openapi-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-pkg "github.com/karmada-io/karmada/operator/pkg/generated/openapi" \
  --output-dir operator/pkg/generated/openapi \
  --output-file zz_generated.openapi.go \
  "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1" \
  "k8s.io/api/core/v1" \
  "k8s.io/apimachinery/pkg/api/resource" \
  "k8s.io/apimachinery/pkg/apis/meta/v1" \
  "k8s.io/apimachinery/pkg/util/intstr" \
  "k8s.io/apimachinery/pkg/runtime" \
  "k8s.io/apimachinery/pkg/version"

EXTERNAL_APPLY_CONFIGS="k8s.io/api/core/v1.Taint:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.Toleration:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.NodeSelector:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.NodeSelectorRequirement:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.NodeSelectorTerm:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.ServiceStatus:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.ResourceQuotaStatus:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.LoadBalancerStatus:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.LoadBalancerIngress:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.ResourceRequirements:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.Affinity:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.PersistentVolumeClaimTemplate:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.HostPathVolumeSource:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.EmptyDirVolumeSource:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.Volume:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.VolumeMount:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/core/v1.Container:k8s.io/client-go/applyconfigurations/core/v1,\
k8s.io/api/networking/v1.IngressSpec:k8s.io/client-go/applyconfigurations/networking/v1,\
k8s.io/api/networking/v1.IngressStatus:k8s.io/client-go/applyconfigurations/networking/v1,\
k8s.io/api/networking/v1.IngressLoadBalancerStatus:k8s.io/client-go/applyconfigurations/networking/v1,\
k8s.io/api/networking/v1.IngressLoadBalancerIngress:k8s.io/client-go/applyconfigurations/networking/v1,\
k8s.io/api/networking/v1.IngressPortStatus:k8s.io/client-go/applyconfigurations/networking/v1,\
k8s.io/api/autoscaling/v2.HorizontalPodAutoscalerStatus:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/autoscaling/v2.CrossVersionObjectReference:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/autoscaling/v2.MetricSpec:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/autoscaling/v2.HorizontalPodAutoscalerBehavior:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/autoscaling/v2.MetricStatus:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/autoscaling/v2.HPAScalingRules:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/autoscaling/v2.HPAScalingPolicy:k8s.io/client-go/applyconfigurations/autoscaling/v2,\
k8s.io/api/admissionregistration/v1.WebhookClientConfig:k8s.io/client-go/applyconfigurations/admissionregistration/v1,\
k8s.io/api/admissionregistration/v1.ServiceReference:k8s.io/client-go/applyconfigurations/admissionregistration/v1"

echo "Generating with applyconfiguration-gen"
applyconfiguration-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-pkg=github.com/karmada-io/karmada/pkg/generated/applyconfigurations \
  --output-dir=pkg/generated/applyconfigurations \
  --openapi-schema=<(go run github.com/karmada-io/karmada/pkg/generated/openapi/cmd/models-schema) \
  --external-applyconfigurations "${EXTERNAL_APPLY_CONFIGS}" \
  github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1
applyconfiguration-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-pkg=github.com/karmada-io/karmada/operator/pkg/generated/applyconfigurations \
  --output-dir=operator/pkg/generated/applyconfigurations \
  --openapi-schema=<(go run github.com/karmada-io/karmada/operator/pkg/generated/openapi/cmd/models-schema) \
  --external-applyconfigurations "${EXTERNAL_APPLY_CONFIGS}" \
  github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1

echo "Generating with client-gen"
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input=github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1,github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha1,github.com/karmada-io/karmada/pkg/apis/work/v1alpha2,github.com/karmada-io/karmada/pkg/apis/config/v1alpha1,github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1,github.com/karmada-io/karmada/pkg/apis/search/v1alpha1,github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1,github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1,github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1 \
  --output-pkg=github.com/karmada-io/karmada/pkg/generated/clientset \
  --output-dir=pkg/generated/clientset \
  --clientset-name=versioned \
  --apply-configuration-package=github.com/karmada-io/karmada/pkg/generated/applyconfigurations
client-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --input-base="" \
  --input=github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1 \
  --output-pkg=github.com/karmada-io/karmada/operator/pkg/generated/clientset \
  --output-dir=operator/pkg/generated/clientset \
  --clientset-name=versioned \
  --apply-configuration-package=github.com/karmada-io/karmada/operator/pkg/generated/applyconfigurations

echo "Generating with lister-gen"
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-pkg=github.com/karmada-io/karmada/pkg/generated/listers \
  --output-dir=pkg/generated/listers \
  github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1
lister-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-pkg=github.com/karmada-io/karmada/operator/pkg/generated/listers \
  --output-dir=operator/pkg/generated/listers \
  github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1

echo "Generating with informer-gen"
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --versioned-clientset-package=github.com/karmada-io/karmada/pkg/generated/clientset/versioned \
  --listers-package=github.com/karmada-io/karmada/pkg/generated/listers \
  --output-pkg=github.com/karmada-io/karmada/pkg/generated/informers \
  --output-dir=pkg/generated/informers \
  github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/work/v1alpha2 \
  github.com/karmada-io/karmada/pkg/apis/config/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/search/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1 \
  github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1
informer-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --versioned-clientset-package=github.com/karmada-io/karmada/operator/pkg/generated/clientset/versioned \
  --listers-package=github.com/karmada-io/karmada/operator/pkg/generated/listers \
  --output-pkg=github.com/karmada-io/karmada/operator/pkg/generated/informers \
  --output-dir=operator/pkg/generated/informers \
  github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1

