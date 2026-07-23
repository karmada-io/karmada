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

CONTROLLER_GEN_PKG="sigs.k8s.io/controller-tools/cmd/controller-gen"
CONTROLLER_GEN_VER="v0.21.0"
YQ_PKG="github.com/mikefarah/yq/v4"
YQ_VER="v4.44.6"

source hack/util.sh

echo "Generating with controller-gen"
util::install_tools ${CONTROLLER_GEN_PKG} ${CONTROLLER_GEN_VER} >/dev/null 2>&1
util::install_tools ${YQ_PKG} ${YQ_VER} >/dev/null 2>&1

# Unify the crds used by helm chart and the installation scripts
controller-gen crd paths=./pkg/apis/autoscaling/... output:crd:dir=./charts/karmada/_crds/bases/autoscaling
controller-gen crd paths=./pkg/apis/config/... output:crd:dir=./charts/karmada/_crds/bases/config
controller-gen crd paths=./pkg/apis/networking/... output:crd:dir=./charts/karmada/_crds/bases/networking
controller-gen crd paths=./pkg/apis/policy/... output:crd:dir=./charts/karmada/_crds/bases/policy
controller-gen crd paths=./pkg/apis/remedy/... output:crd:dir=./charts/karmada/_crds/bases/remedy
controller-gen crd paths=./pkg/apis/work/... output:crd:dir=./charts/karmada/_crds/bases/work
controller-gen crd paths=./pkg/apis/apps/... output:crd:dir=./charts/karmada/_crds/bases/apps
controller-gen crd:generateEmbeddedObjectMeta=true paths=./examples/customresourceinterpreter/apis/... output:crd:dir=./examples/customresourceinterpreter/apis/
# Generate operator CRDs to a temp directory, then post-process into a Helm template.
# This pattern is adopted from the cert-manager project.
OPERATOR_CRD_TMP="./charts/karmada-operator/_crds_tmp"
OPERATOR_CRD_TEMPLATE="./charts/karmada-operator/templates/crd-operator.karmada.io_karmadas.yaml"

controller-gen crd:generateEmbeddedObjectMeta=true paths=./operator/pkg/apis/operator/... output:crd:dir="${OPERATOR_CRD_TMP}"

CRD_FILE="${OPERATOR_CRD_TMP}/operator.karmada.io_karmadas.yaml"
CRD_NAME=$(yq eval '.metadata.name' "${CRD_FILE}")
CONTROLLER_GEN_VERSION=$(yq eval '.metadata.annotations["controller-gen.kubebuilder.io/version"]' "${CRD_FILE}")

cat > "${OPERATOR_CRD_TEMPLATE}" <<HEADER
{{- if .Values.installCRDs }}
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: "${CRD_NAME}"
  annotations:
    controller-gen.kubebuilder.io/version: ${CONTROLLER_GEN_VERSION}
    {{- if .Values.crds.keep }}
    helm.sh/resource-policy: keep
    {{- end }}
HEADER
yq eval '{"spec": .spec}' "${CRD_FILE}" >> "${OPERATOR_CRD_TEMPLATE}"
echo '{{- end }}' >> "${OPERATOR_CRD_TEMPLATE}"

rm -rf "${OPERATOR_CRD_TMP}"

controller-gen crd:generateEmbeddedObjectMeta=true paths=./operator/pkg/apis/operator/... output:crd:dir=./operator/config/crds
