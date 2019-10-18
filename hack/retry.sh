#!/usr/bin/env bash
if [ $# -lt 3 ]; then
    echo "USAGE: ${BASH_SOURCE} retryCount retryIntervalInSeconds command..."
    exit 1
fi

retryCount=$1
shift
retryInterval=$1
shift

commandToRetry=$@

until [ ${retryCount} -eq 0 ]; do
    ${commandToRetry} && break
    retryCount=$[$retryCount-1]
    echo "Failed, retrying in ${retryInterval} seconds, ${retryCount} retries left"
    sleep ${retryInterval}
done

if [ ${retryCount} -eq 0 ]; then
    exit 1
fi
