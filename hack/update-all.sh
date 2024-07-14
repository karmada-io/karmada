#!/usr/bin/env bash
# Copyright 2021 The Karmada Authors.
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
