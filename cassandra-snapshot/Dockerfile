FROM alpine:3.10.2

RUN adduser -S -u 999 cassandra-operator
COPY build/bin/cassandra-snapshot /cassandra-snapshot

ENTRYPOINT ["/cassandra-snapshot"]
