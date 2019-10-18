#!/usr/bin/env bash
context=$1
namespace=$2
matcher=$3
expectedStatus=$4

kubectl --context ${context} -n ${namespace} get pod -l${matcher} -o json | jq --exit-status ".items[0].status.containerStatuses[0].state[\"waiting\"].reason == \"${expectedStatus}\""
