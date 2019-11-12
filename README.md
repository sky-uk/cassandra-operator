# Cassandra Operator [![Build Status](https://travis-ci.com/sky-uk/cassandra-operator.svg?branch=master)](https://travis-ci.com/sky-uk/cassandra-operator)

The Cassandra Operator is a Kubernetes operator that manages Cassandra clusters inside Kubernetes.

The project is `alpha` status and can be used in development environments.
It is not yet recommended for use in production environments.

## Main features

* rack awareness
* scaling out (more racks, more pods per rack)
* scheduled backups with retention policy
* periodic cluster definition reconciliation
* lighweight nodetool status equivalent probes  
* works with official Cassandra Docker images
* deployable per namespace with RBAC permissions limited to it
* deployable cluster-wide
* customisable Cassandra config (`cassandra.yaml`, `jvm.options`, extra libs)
* customisable liveness / readiness probes
* automated rolling update of Cassandra cluster definition changes
* interruption of in-progress reconciliations when newer changes are detected
* cluster and node level metrics
* a comprehensive e2e test suite

## How to use it?

Instructions on how to deploy the Cassandra Operator and provision Cassandra clusters can be found on the [WIKI](https://github.com/sky-uk/cassandra-operator/wiki)  

## Project structure

This project is composed of several sub-modules that are either part of the Cassandra Operator or used by it:
- [cassandra-operator](cassandra-operator/README.md): the Kubernetes operator that manages the Cassandra clusters lifecycle inside Kubernetes
- [cassandra-bootstrapper](cassandra-bootstrapper/README.md): a component responsible for configuring the Cassandra node before it can be started
- [cassandra-sidecar](cassandra-sidecar/README.md): a sidecar container running on each Cassandra node exposing node status 
- [cassandra-snapshot](cassandra-snapshot/README.md): a component responsible for taking and deleting snapshots given a schedule and retention policy
- [fake-cassandra-docker](fake-cassandra-docker/README.md): a fake Cassandra image used by the cassandra-operator and cassandra-snapshot to speed it up end-to-end testing
- [test-kubernetes-cluster](test-kubernetes-cluster/README.md): a [Kind](https://kind.sigs.k8s.io/) cluster used by the cassandra-operator and cassandra-snapshot to facilitate end-to-end testing

## Design

The Cassandra Operator and the components it uses are described here: [Cassandra Operator Design](design/cassandra-operator-design.md) 

## Supported versions

We test the Cassandra Operator against the following Kubernetes / Cassandra versions.

Other Kubernetes versions are likely to work, but we do not actively test against them.

Cassandra Operator | Kubernetes | Cassandra
--- | --- | ---
0.77.0-alpha | 1.12 | 3.11
0.75.0-alpha | 1.11 | 3.11
0.70.1-alpha | 1.10 | 3.11

## Questions or Problems?

- If you have a general question about this project, please create an issue for it. The issue title should be the
  question itself, with any follow-up information in a comment. Add the "question" tag to the issue.

- If you think you have found a bug in this project, please create an issue for it. Use the issue title to summarise
  the problems, and supply full steps to reproduce in a comment. Add the "bug" tag to the issue.

## Contributions

See [CONTRIBUTING.md](CONTRIBUTING.md)
