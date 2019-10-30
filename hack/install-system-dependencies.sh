#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR=$(dirname ${BASH_SOURCE})
ROOT_DIR="$(cd ${SCRIPT_DIR}/.. && pwd)"
: ${BIN_DIR:="${ROOT_DIR}/bin"}

mkdir -p "${BIN_DIR}"

pushd "${BIN_DIR}"
curl --silent \
     --show-error \
     --location \
     --output kubectl \
     https://storage.googleapis.com/kubernetes-release/release/v1.13.12/bin/linux/amd64/kubectl \
     --output goss \
     https://github.com/aelsabbahy/goss/releases/download/v0.3.7/goss-linux-amd64 \
     --output dgoss \
     https://github.com/aelsabbahy/goss/releases/download/v0.3.7/dgoss \
     --output kind \
     https://github.com/kubernetes-sigs/kind/releases/download/v0.5.1/kind-linux-amd64
chmod +x kubectl goss dgoss kind
