#!/usr/bin/env bash
set -e

scriptDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
resourcesDir="${scriptDir}/../kubernetes-resources"

function waitForPod {
    local count=0
    local sleepInterval=5
    local maxRetry=60 #5mins max
    local context=$1
    local namespace=$2
    local pod=$3

    ready=""
    until [[ ${ready} = "0" ]] || (( "$count" >= "$maxRetry" ))
    do
        count=$((count+1))
        echo "Waiting for pod: $pod in namespace: $namespace. Attempt: $count"
        status=$(kubectl --context ${context} -n ${namespace} get po ${pod} -o go-template="{{ range .status.conditions }}{{ .type }}={{ .status }} {{ end }}" || true)
        set +e
        echo "$status" | grep "Ready=True"
        ready=$?
        set -e
        sleep ${sleepInterval}
    done

    if [[ ${ready} != "0" ]]; then
        echo "Pod ${pod} failed to become ready after ${maxRetry} retries"
        exit 1
    fi
    echo "Pod ${pod} is ready"
}

function waitForJobToComplete {
    local context=$1
    local namespace=$2
    local job=$3
    local count=0
    local sleepInterval=5
    local maxRetry=120 #10mins max
    
    echo "Waiting for the job completion"
    complete=""
    until [[ ${complete} = "0" ]] || (( "$count" >= "$maxRetry" ))
    do 
        count=$((count+1))
        echo "Waiting for job: $job in namespace: $namespace. Attempt: $count"
        status=$(kubectl --context ${context} -n ${namespace} get jobs ${job} -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' || true)
        set +e
        echo "$status" | grep "True"
        complete=$?
        set -e
        sleep ${sleepInterval} 
    done
    
    if [[ ${complete} != "0" ]]; then
        echo "Job ${job} failed to complete after ${maxRetry} retries"
        exit 1
    fi
    echo "Job ${job} is complete"
}

function createCluster {
    local context=$1
    local namespace=$2
    local bootstrapperImage=$3
    local sidecarImage=$4
    local clusterName=$5
    local tmpDir=$(mktemp -d)
    trap '{ CODE=$?; rm -rf ${tmpDir} ; exit ${CODE}; }' EXIT

    echo "Creating cluster $clusterName"

    k8Resource="$clusterName.yml"
    sed -e "s@\$TARGET_NAMESPACE@$namespace@g" \
        -e "s@\$CASSANDRA_BOOTSTRAPPER_IMAGE@$bootstrapperImage@g" \
        -e "s@\$CASSANDRA_SIDECAR_IMAGE@$sidecarImage@g" \
        ${resourcesDir}/${k8Resource} > ${tmpDir}/${k8Resource}
    kubectl --context ${context} -n ${namespace} apply -f ${tmpDir}/${k8Resource}

    for pod in ${clusterName}-a-0 ${clusterName}-a-1 ${clusterName}-b-0 ${clusterName}-b-1
    do
        waitForPod ${context} ${namespace} ${pod}
    done
    echo "Cluster is ready"
}

function runLoadTest {
    local context=$1
    local namespace=$2
    local nftImage=$3
    local clusterName=$4
    local jobTimeoutSeconds=500
    local contactPoints="${clusterName}.${namespace}.svc.cluster.local."

    local tmpDir=$(mktemp -d)
    trap '{ CODE=$?; rm -rf ${tmpDir} ; exit ${CODE}; }' EXIT

    k8Resource="nft-job.yml"
    sed -e "s@\$TARGET_NAMESPACE@$namespace@g" \
        -e "s@\$NFT_IMAGE@$nftImage@g" \
        -e "s@\$ACTIVE_DEADLINE_SECONDS@$jobTimeoutSeconds@g" \
        -e "s@\$CONTACT_POINTS@$contactPoints@g" \
        ${resourcesDir}/${k8Resource} > ${tmpDir}/${k8Resource}
    kubectl --context ${context} -n ${namespace} delete job nft --ignore-not-found
    kubectl --context ${context} -n ${namespace} apply -f ${tmpDir}/${k8Resource}

    waitForJobToComplete ${context} ${namespace} nft
}


usage="Usage: CONTEXT=<context> NAMESPACE=<namespace> CASSANDRA_BOOTSTRAPPER_IMAGE=<boostrapperImage> CASSANDRA_SIDECAR_IMAGE=<sidecarImage> NFT_IMAGE=<nftImage> $0"
: ${CASSANDRA_BOOTSTRAPPER_IMAGE?${usage}}
: ${CASSANDRA_SIDECAR_IMAGE?${usage}}
: ${NFT_IMAGE?${usage}}
: ${CONTEXT?${usage}}
: ${NAMESPACE?${usage}}


echo "Creating the cluster"
createCluster ${CONTEXT} ${NAMESPACE} ${CASSANDRA_BOOTSTRAPPER_IMAGE} ${CASSANDRA_SIDECAR_IMAGE} small-cluster

echo "Running the load test"
runLoadTest ${CONTEXT} ${NAMESPACE} ${NFT_IMAGE} small-cluster

echo "NFT complete"
