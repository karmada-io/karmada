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
bash "$REPO_ROOT/hack/verify-license.sh"
