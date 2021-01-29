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

if [[ $projectDir != *$basePackage* ]]; then
  echo "Please ensure cassandra-operator is cloned into github.com/sky-uk/cassandra-operator to ensure generated files are created in the right places"
  exit 1
fi

chmod +x ${codeGenDir}/generate-groups.sh

# Ensure that cassandra-operator is cloned under you GOPATH like so
# $GOPATH/github.com/sky-uk/cassandra-operator
# This is to ensure code-generator can be run from inside the vendor directory.
${codeGenDir}/generate-groups.sh all \
  ${basePackage}/pkg/client ${basePackage}/pkg/apis \
  ${groupVersion} \
  --go-header-file ${scriptDir}/empty-boilerplate.txt \
  --output-base ${projectDir}/../../../..
