#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

version=$1

release_dir="${REPO_ROOT}/_output/release"
tar_file="karmada-chart-${version}.tgz"
mkdir -p "${release_dir}"

# install helm
echo -n "Preparing: 'helm' existence check - "
if util::cmd_exist helm; then
  echo "passed"
else
  echo "installing helm"
  util::install_helm
fi

echo "Starting to package into a Karmada chart archive"
helm package ./charts --version "${version}" -d "${release_dir}"
cd "${release_dir}"
mv "karmada-${version}.tgz" ${tar_file}
sha256sum "${tar_file}" > "${tar_file}.sha256"
