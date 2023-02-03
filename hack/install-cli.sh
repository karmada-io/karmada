#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# This script installs karmada cli tools to the install location you want.
# Usage: ./install-cli.sh <install_cli_type>
# Example:

# ./install-cli.sh
# will install the latest karmadactl to /usr/local/bin

# ./install-cli.sh kubectl-karmada
# will install the latest kubectl-karmada to /usr/local/bin

# export INSTALL_CLI_VERSION=1.3.0
# ./install-cli.sh
# will install the 1.3.0 karmadactl to /usr/local/bin

# INSTALL_CLI_TYPE supports 'karmadactl' or 'kubectl-karmada'.
INSTALL_CLI_TYPE=${1:-"karmadactl"}
# INSTALL_CLI_VERSION represents which version should be installed.
# Defaults to 'latest' means latest release version.
INSTALL_CLI_VERSION=${INSTALL_CLI_VERSION:-"latest"}
# INSTALL_LOCATION represents where to put the CLI.
INSTALL_LOCATION=${INSTALL_LOCATION:-"/usr/local/bin"}
GITHUB_REPO="karmada-io/karmada"

# Helper functions for logs
info() {
  echo '[INFO] ' "$@"
}

warn() {
  echo '[WARN] ' "$@" >&2
}

fatal() {
  echo '[ERROR] ' "$@" >&2
  exit 1
}

# Set os, fatal if operating system not supported
setup_verify_os() {
  if [[ -z "${OS-}" ]]; then
    OS=$(uname)
  fi
  case ${OS} in
    Darwin)
      OS=darwin
      ;;
    Linux)
      OS=linux
      ;;
    *)
      fatal "Unsupported operating system ${OS}"
  esac
}

# Set arch, fatal if architecture not supported
setup_verify_arch() {
  if [[ -z "${ARCH-}" ]]; then
    ARCH=$(uname -m)
  fi
  case ${ARCH} in
    arm64|aarch64|armv8l)
      ARCH=arm64
      ;;
    amd64|x86_64)
      ARCH=amd64
      ;;
    *)
      fatal "Unsupported architecture ${ARCH}"
  esac
}

# Set cli tools, install karmadactl by default, fatal if cli tool not supported
setup_install_cli() {
  case ${INSTALL_CLI_TYPE} in
    karmadactl|kubectl-karmada)
      ;;
    *)
      fatal "Unsupported cli tool ${INSTALL_CLI_TYPE}"
  esac
}

# Verify existence of downloader executable
verify_downloader() {
  # Return failure if it doesn't exist or is no executable
  [[ -x "$(which "$1")" ]] || return 1

  # Set verified executable as our downloader program and return success
  DOWNLOADER=$1
  return 0
}

# Create tempory directory and cleanup when done
setup_tmp() {
  TMP_DIR=$(mktemp -d -t "${INSTALL_CLI_TYPE}"-install.XXXXXXXXXX)
  TMP_METADATA="${TMP_DIR}/${INSTALL_CLI_TYPE}.json"
  TMP_HASH="${TMP_DIR}/${INSTALL_CLI_TYPE}.hash"
  TMP_BIN="${TMP_DIR}/${INSTALL_CLI_TYPE}.tgz"
  cleanup() {
    local code=$?
    rm -rf "${TMP_DIR}" || true
    exit ${code}
  }
  trap cleanup INT EXIT
}

# Find version from Github metadata
get_release_version() {
  if [[ "${INSTALL_CLI_VERSION-}" != "latest" ]]; then
    SUFFIX_URL="tags/v${INSTALL_CLI_VERSION}"
  else
    SUFFIX_URL="latest"
  fi

  METADATA_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/${SUFFIX_URL}"

  info "Downloading metadata ${METADATA_URL}"
  download "${TMP_METADATA}" "${METADATA_URL}"

  VERSION_KARMADA=$(grep '"tag_name":' "${TMP_METADATA}" | sed -E 's/.*"([^"]+)".*/\1/' | cut -c 2-)
  if [[ -n "${VERSION_KARMADA}" ]]; then
    info "Using ${VERSION_KARMADA} as release"
  else
    fatal "Unable to determine release version"
  fi

  ret=0
  vercomp ${VERSION_KARMADA} 1.2.0 || ret=$?
  if [[ $ret -eq 2 ]]; then
    fatal "Installation of cli tools before version 1.2.0 is not supported"
  fi
}

# Download from file from URL
download() {
  [[ $# -eq 2 ]] || fatal 'download needs exactly 2 arguments'

  ret=0
  case $DOWNLOADER in
    curl)
      curl -o "$1" -sfL "$2" || ret=$?
      ;;
    wget)
      wget -qO "$1" "$2" || ret=$?
      ;;
    *)
      fatal "Incorrect executable '${DOWNLOADER}'"
      ;;
    esac

  # Abort if download command failed
  [[ $ret -eq 0 ]] || fatal 'Download failed'
}

# Version comparison
# Returns 0 on '=', 1 on '>', and 2 on '<'.
# Ref: https://stackoverflow.com/a/4025065
vercomp () {
  if [[ $1 == $2 ]]
  then
    return 0
  fi
  local IFS=.
  local i ver1=($1) ver2=($2)
  # fill empty fields in ver1 with zeros
  for ((i=${#ver1[@]}; i<${#ver2[@]}; i++))
  do
    ver1[i]=0
  done
  for ((i=0; i<${#ver1[@]}; i++))
  do
    if [[ -z ${ver2[i]} ]]
    then
      # fill empty fields in ver2 with zeros
      ver2[i]=0
    fi
    if ((10#${ver1[i]} > 10#${ver2[i]}))
    then
      return 1
    fi
    if ((10#${ver1[i]} < 10#${ver2[i]}))
    then
      return 2
    fi
  done
  return 0
}

# Download hash from Github URL
download_hash() {
  HASH_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION_KARMADA}/${INSTALL_CLI_TYPE}-${OS}-${ARCH}.tgz.sha256"

  info "Downloading hash ${HASH_URL}"
  download "${TMP_HASH}" "${HASH_URL}"
  HASH_EXPECTED=$(grep "${INSTALL_CLI_TYPE}-${OS}-${ARCH}.tgz" "${TMP_HASH}")
  HASH_EXPECTED=${HASH_EXPECTED%%[[:blank:]]*}
}

# Download binary from Github URL
download_binary() {
  BIN_URL="https://github.com/${GITHUB_REPO}/releases/download/v${VERSION_KARMADA}/${INSTALL_CLI_TYPE}-${OS}-${ARCH}.tgz"
  info "Downloading binary ${BIN_URL}"
  download "${TMP_BIN}" "${BIN_URL}"
}

compute_sha256sum() {
  cmd=$(which sha256sum shasum | head -n 1)
  case $(basename "$cmd") in
    sha256sum)
      sha256sum "$1" | cut -f 1 -d ' '
      ;;
    shasum)
      shasum -a 256 "$1" | cut -f 1 -d ' '
      ;;
    *)
      fatal "Can not find sha256sum or shasum to compute checksum"
      ;;
  esac
}

# Verify downloaded binary hash
verify_binary() {
  info "Verifying binary download"
  HASH_BIN=$(compute_sha256sum "${TMP_BIN}")
  HASH_BIN=${HASH_BIN%%[[:blank:]]*}
  if [[ "${HASH_EXPECTED}" != "${HASH_BIN}" ]]; then
    fatal "Download sha256 does not match ${HASH_EXPECTED}, got ${HASH_BIN}"
  fi
}

# Setup permissions and move binary
setup_binary() {
  info "Installing ${INSTALL_CLI_TYPE} to ${INSTALL_LOCATION}/${INSTALL_CLI_TYPE}"
  tar -xzof "${TMP_BIN}" -C "${TMP_DIR}" --no-same-owner

  mkdir -p "${INSTALL_LOCATION}" &>/dev/null || sudo mkdir -p "${INSTALL_LOCATION}"

  local CMD_MOVE="mv -i \"${TMP_DIR}/${INSTALL_CLI_TYPE}\" \"${INSTALL_LOCATION}\""
  if [[ -w "${INSTALL_LOCATION}" ]]; then
    eval "${CMD_MOVE}"
  else
    eval "sudo ${CMD_MOVE}"
  fi
}

# Run the install process
{
  setup_verify_os
  setup_verify_arch
  setup_install_cli
  verify_downloader curl || verify_downloader wget || fatal 'Can not find curl or wget for downloading files'
  setup_tmp
  get_release_version
  download_hash
  download_binary
  verify_binary
  setup_binary
}
