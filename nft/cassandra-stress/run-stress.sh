#!/usr/bin/env bash
set -e

function log {
    local message=$1
    local datetime=$(date +"%Y-%m-%dT%T")
    echo "[Cassandra NFT] $datetime: $message"
}

function usage {
    echo "Usage: CONTACT_POINTS=<cassandra node> TEST_SEGMENT_DURATION=<segment run duration in secs/hours> SLEEP_TIME_SECONDS_BETWEEN_PROFILE=<sleep time between profiles> $0"
}

: ${CONTACT_POINTS?usage}
: ${TEST_SEGMENT_DURATION?usage}
: ${SLEEP_TIME_SECONDS_BETWEEN_PROFILE?usage}

export JVM_OPTS="-Xms3072m -Xmx3072m $JVM_OPTS"

log "Starting tests with contact points=${CONTACT_POINTS}, test duration=${TEST_SEGMENT_DURATION}"
log "Data load"

cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1\) n=100000 cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -rate threads=64

log "Pause ${SLEEP_TIME_SECONDS_BETWEEN_PROFILE}s before test"
sleep ${SLEEP_TIME_SECONDS_BETWEEN_PROFILE}

log "Starting UPDATE HEAVY R/W=1:1 on"
cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=1\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=120/s
cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=1\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=720/s
cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=1\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=4320/s
cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=1\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=8640/s
#sleep ${SLEEP_TIME_SECONDS_BETWEEN_PROFILE}
#
#log "Starting READ HEAVY R/W=19:1 on"
#cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=19\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=120/s
#cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=19\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=720/s
#cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=19\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=4320/s
#cassandra-stress user profile=/opt/cassandra-stress/profile.yaml ops\(insert=1,alldevices=19\) duration=${TEST_SEGMENT_DURATION} cl=local_quorum -node ${CONTACT_POINTS} -mode native cql3 -errors retries=100 -rate threads=64 throttle=8640/s
#sleep ${SLEEP_TIME_SECONDS_BETWEEN_PROFILE}

log "Tests completed"
