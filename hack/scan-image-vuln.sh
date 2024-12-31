#!/usr/bin/env bash
# Copyright 2024 The Karmada Authors.
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

# This script starts a images scanning with trivy
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. used to locally scan Karmada component images vulnerabilities with trivy
# 2. Used to scan specified image with trivy

function usage() {
    echo "Usage:"
    echo "    hack/scan-image-vuln.sh [-i imageRef] [-r registry] [-v version] [-s skip-image-generation] [-f format][-h]"
    echo "Examples:"
    echo "    # starts a images scanning with specific image provided"
    echo "    hack/scan-image-vuln.sh -i docker.io/karmada/karmada-controller-manager:v1.8.0"
    echo "    # scan Karmada component images with trivy and images will be automatically generated, imageRef='docker.io/karmada/{imageName}:latest'"
    echo "    hack/scan-image-vuln.sh"
    echo "    # scan Karmada component images with trivy and images generation will be skipped, imageRef='docker.io/karmada/{imageName}:latest'"
    echo "    hack/scan-image-vuln.sh -s"
    echo "    # scan Karmada component images with trivy and provide specific image's registry or version"
    echo "    hack/scan-image-vuln.sh -r foo # imageRef='foo/{imageName}:latest'"
    echo "    hack/scan-image-vuln.sh -s -v v1.8.0 # imageRef='docker.io/karmada/{imageName}:v1.8.0'"
    echo "Args:"
    echo "    i imageRef: starts a images scanning with specific image provided, if not provided, local Karmada images will be scanned"
    echo "    r registry: registry of images"
    echo "    v version: version of images"
    echo "    s skip-image-generation: whether to skip image generation"
    echo "    f format: output format(table). must be one of ['table' 'json' 'template' 'sarif' 'cyclonedx' 'spdx' 'spdx-json' 'github' 'cosign-vuln']"
    echo "    h: print help information"
}

while getopts 'h:si:r:v:f:' OPT; do
    case $OPT in
        h)
          usage
          exit 0
          ;;
        s)
        	SKIP_IMAGE_GENERAION="true";;
        i)
        	IMAGEREF=${OPTARG};;
       	r)
       		REGISTRY=${OPTARG};;
        v)
        	VERSION=${OPTARG};;
        f)
        	FORMAT=${OPTARG};;
        ?)
          usage
          exit 1
          ;;
    esac
done

FORMAT=${FORMAT:-"table"}
SKIP_IMAGE_GENERAION=${SKIP_IMAGE_GENERAION:-"false"}
IMAGEREF=${IMAGEREF:-""}

source "hack/util.sh"

echo -n "Preparing: 'trivy' existence check - "
if util::cmd_exist trivy ; then
	echo "pass"
else
	echo "start installing trivy"
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v0.56.1
fi

if [ ${IMAGEREF} ];then
	echo "---------------------------- the image scanning result of Image <<${IMAGEREF}>> ----------------------------"
  trivy image --format ${FORMAT} --ignore-unfixed --vuln-type os,library --severity UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL -q ${IMAGEREF}
  exit 0
fi

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}"
export VERSION=${VERSION:-"latest"}
export REGISTRY=${REGISTRY:-"docker.io/karmada"}
IMAGE_ARRAR=(
	karmada-controller-manager
  karmada-scheduler
  karmada-descheduler
  karmada-webhook
  karmada-agent
  karmada-scheduler-estimator
  karmada-interpreter-webhook-example
  karmada-aggregated-apiserver
  karmada-search
  karmada-operator
  karmada-metrics-adapter
)
if [ ${SKIP_IMAGE_GENERAION} == "false" ]; then
	echo "start generating image"
	make images GOOS="linux" --directory=.
fi

echo "start image scan"
for image in ${IMAGE_ARRAR[@]}
do
  imageRef="$REGISTRY/$image:$VERSION"
  echo "---------------------------- the image scanning result of Image <<$imageRef>> ----------------------------"
  trivy image --format ${FORMAT} --ignore-unfixed --vuln-type os,library --severity UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL -q $imageRef
done
