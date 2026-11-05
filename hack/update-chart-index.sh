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

# This script used to update helm index to specific release version and automatically submit a pr to remote repo.
# Usage:
#   export CURRENT_REPO_ORG=karmada-io CURRENT_REPO_NAME=karmada tag=v1.12.0
#   hack/update-helm-index.sh

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd ${REPO_ROOT}

CURRENT_REMOTE=${CURRENT_REMOTE:-origin}
CURRENT_REPO_ORG=${CURRENT_REPO_ORG:-$(git remote get-url "$CURRENT_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $3}')}
CURRENT_REPO_NAME=${CURRENT_REPO_NAME:-$(git remote get-url "$CURRENT_REMOTE" | awk '{gsub(/http[s]:\/\/|git@/,"")}1' | awk -F'[@:./]' 'NR==1{print $4}')}

get_latest_release_tag() {
  curl --silent "https://api.github.com/repos/$1/releases/latest" |
    grep '"tag_name":' |
    sed -E 's/.*"([^"]+)".*/\1/'
}

# step1: get tag, defaults to latest release tag
tag=${tag:-"$(get_latest_release_tag "${CURRENT_REPO_ORG}/${CURRENT_REPO_NAME}")"}
if [ $(grep -c "version: ${tag}" charts/index.yaml) -ge '2' ]; then
  echo "the tag already in helm index!"
  exit 0
fi

# step2: checkout a new branch
# Normally return if remote branch already exist, which means you can re-execute the script without throwing errors.
NEWBRANCH="auto-helm-index-${tag}"
git fetch -q
if [ $(git branch -r | grep -c "origin/${NEWBRANCH}") -gt '0' ]; then
  echo 'remote branch '${NEWBRANCH}' already exist!'
  exit 0
fi
git checkout -b ${NEWBRANCH}

# step3: update index for karmada-chart
wget https://github.com/${CURRENT_REPO_ORG}/${CURRENT_REPO_NAME}/releases/download/${tag}/karmada-chart-${tag}.tgz -P charts/karmada/
helm repo index charts/karmada --url https://github.com/${CURRENT_REPO_ORG}/${CURRENT_REPO_NAME}/releases/download/${tag} --merge charts/index.yaml
mv charts/karmada/index.yaml charts/index.yaml

# step4: update index for karmada-operator-chart
wget https://github.com/${CURRENT_REPO_ORG}/${CURRENT_REPO_NAME}/releases/download/${tag}/karmada-operator-chart-${tag}.tgz -P charts/karmada-operator/
helm repo index charts/karmada-operator --url https://github.com/${CURRENT_REPO_ORG}/${CURRENT_REPO_NAME}/releases/download/${tag} --merge charts/index.yaml
mv charts/karmada-operator/index.yaml charts/index.yaml

# step5: the `helm repo index` command also generates index for dependencies(common-2.x.x) by default,
# which is undesirable; therefore, the contents of the `common` field should be manaually removed.
# this `sed` command deletes lines between the line `entries:` and the line `karmada:`.
sed -i'' '/entries:/,/karmada:/{//!d}' charts/index.yaml
echo "Successfully generated helm index."

# step6: commit the modification
git add charts/index.yaml
git commit -s -m "Bump upgrade helm chart index to ${tag}"
git push origin ${NEWBRANCH}
echo "Successfully pushed the commit."

# step6: create pull request
prtext=$(
  cat <<EOF
**What type of PR is this?**

/kind cleanup

**What this PR does / why we need it**:

Bump upgrade helm chart index to ${tag}

**Which issue(s) this PR fixes**:

Fixes

**Does this PR introduce a user-facing change?**:
\`\`\`release-note
upgrade helm chart index to ${tag}.
\`\`\`
EOF
)
gh pr create --title "Bump upgrade helm chart index to ${tag}" --body "${prtext}" --base master --head "${NEWBRANCH}" --repo="${CURRENT_REPO_ORG}/${CURRENT_REPO_NAME}"
echo "Successfully created the pr."
