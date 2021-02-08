#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

scriptDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
projectDir="${scriptDir}/.."
codeGenDir="${projectDir}/vendor/k8s.io/code-generator"
basePackage="github.com/sky-uk/cassandra-operator/cassandra-operator"
groupVersion="cassandra:v1alpha1"
tempDir="$(mktemp -d)"
generateDeepCopy="/pkg/apis/cassandra/v1alpha1/zz_generated.deepcopy.go"
pkgClientPath="/pkg/client"

cleanup() {
  rm -rf "${tempDir}"
}
trap "cleanup" EXIT SIGINT

mkdir -p "${tempDir}/${basePackage}}"

# ensure code-generator is available in vendor directory
# https://github.com/kubernetes/code-generator/issues/57#issuecomment-498310800
cd ${projectDir}
go mod vendor

chmod +x ${codeGenDir}/generate-groups.sh

${codeGenDir}/generate-groups.sh all \
  ${basePackage}/pkg/client ${basePackage}/pkg/apis \
  ${groupVersion} \
  --go-header-file ${scriptDir}/empty-boilerplate.txt \
  --output-base ${tempDir}


cp "${tempDir}/${basePackage}/${generateDeepCopy}" "${projectDir}/${generateDeepCopy}"
cp -ur "${tempDir}/${basePackage}/${pkgClientPath}" "${projectDir}/pkg/"

