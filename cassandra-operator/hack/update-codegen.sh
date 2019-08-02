#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

scriptDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
projectDir="${scriptDir}/.."
codeGenDir="${projectDir}/vendor/k8s.io/code-generator"
basePackage="github.com/sky-uk/cassandra-operator/cassandra-operator"
groupVersion="cassandra:v1alpha1"

# ensure code-generator is available in vendor directory
# https://github.com/kubernetes/code-generator/issues/57#issuecomment-498310800
cd ${projectDir}
go mod vendor

chmod +x ${codeGenDir}/generate-groups.sh

${codeGenDir}/generate-groups.sh all \
  ${basePackage}/pkg/client ${basePackage}/pkg/apis \
  ${groupVersion} \
  --go-header-file ${scriptDir}/empty-boilerplate.txt
