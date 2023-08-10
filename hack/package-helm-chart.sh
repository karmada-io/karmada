#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

version=${1}
output_dir="${REPO_ROOT}/_output/charts"

KARMADA_CHARTS=(
    karmada
    karmada-operator
)

mkdir -p "${output_dir}"

# install helm
echo -n "Preparing: 'helm' existence check - "
if util::cmd_exist helm; then
  echo "passed"
else
  echo "installing helm"
  util::install_helm
fi

tar_file=""
for chart in ${KARMADA_CHARTS[@]}; 
do
    sed -i'' -e "s/\&karmadaImageVersion .*/\&karmadaImageVersion ${version}/g" ./charts/"${chart}"/values.yaml

    tar_file="${chart}-chart-${version}.tgz"
    echo "Starting to package into a ${chart} chart archive"
    helm package ./charts/"${chart}" --version "${version}" -d "${output_dir}" -u

    echo "Rename ${chart}-${version}.tgz to ${tar_file}"
    mv "${output_dir}/${chart}-${version}.tgz" "${output_dir}/${tar_file}"

    sha256sum "${output_dir}/${tar_file}" > "${output_dir}/${tar_file}.sha256"
done

