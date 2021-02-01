#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

scriptDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
projectDir="${scriptDir}/.."
codeGenDir="${projectDir}/vendor/k8s.io/code-generator"
basePackage="github.com/sky-uk/cassandra-operator/cassandra-operator"
groupVersion="cassandra:v1alpha1"
VENDOR_DIR="${projectDir}/vendor"
GEN_FILE_PATH="/pkg/apis/cassandra/v1alpha1/zz_generated.deepcopy.go"
CLIENT_PATH="/pkg/client"

# ensure code-generator is available in vendor directory
# https://github.com/kubernetes/code-generator/issues/57#issuecomment-498310800
cd ${projectDir}
go mod vendor

chmod +x ${codeGenDir}/generate-groups.sh

${codeGenDir}/generate-groups.sh all \
  ${basePackage}/pkg/client ${basePackage}/pkg/apis \
  ${groupVersion} \
  --go-header-file ${scriptDir}/empty-boilerplate.txt \
  --output-base ${VENDOR_DIR}


cp "${VENDOR_DIR}/${basePackage}/${GEN_FILE_PATH}" "${projectDir}/${GEN_FILE_PATH}"
cp -ur "${VENDOR_DIR}/${basePackage}/${CLIENT_PATH}" "${projectDir}/pkg/"

