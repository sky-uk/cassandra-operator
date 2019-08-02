#!/usr/bin/env bash

# based off https://github.com/jetstack/cert-manager/blob/master/hack/verify-crds.sh

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd -P)"/..

output="$(mktemp -d)"

cleanup() {
  rm -rf "${output}"
}
trap "cleanup" EXIT SIGINT

tmp="${output}/cassandra-operator"


rsync -avvL "${REPO_ROOT}"/ "${tmp}" >/dev/null
cd "${tmp}"
"./hack/update-codegen.sh"

echo "diffing against freshly generated codegen (${tmp})"
deepCopyDiff=0
diff -Naupr "${REPO_ROOT}/pkg/apis/cassandra/v1alpha1/zz_generated.deepcopy.go" "${tmp}/pkg/apis/cassandra/v1alpha1/zz_generated.deepcopy.go" || deepCopyDiff=$?
clientDiff=0
diff -Naupr "${REPO_ROOT}/pkg/client" "${tmp}/pkg/client" || clientDiff=$?
if [[ ${deepCopyDiff} -eq 0 ]] || [[ ${clientDiff} -eq 0 ]]
then
  echo "${REPO_ROOT} up to date."
else
  echo "${REPO_ROOT} is out of date. Please run ./hack/update-codegen.sh"
  exit 1
fi
