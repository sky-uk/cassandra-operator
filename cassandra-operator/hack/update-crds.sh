#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd -P)"/..
cd "${REPO_ROOT}"

crd_path="${REPO_ROOT}/kubernetes-resources/cassandra-operator-crd.yml"

output="$(mktemp -d)"
controller-gen crd:trivialVersions=true paths=./pkg/apis/... output:crd:dir="${output}"
mv "${output}"/core.sky.uk_cassandras.yaml "${crd_path}"
