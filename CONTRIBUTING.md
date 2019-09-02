# Contributing

Contributions are welcomed!

When contributing to this repository, please first discuss the change you wish to make via a GitHub
issue before making a change.  This saves everyone from wasted effort in the event that the proposed
changes need some adjustment before they are ready for submission.
All new code, including changes to existing code, should be tested and have a corresponding test added or updated where applicable.


## Prerequisites

The following must be installed on your development machine:

- `go` (>=1.12)
- `docker`
- `openjdk-8` or another JDK
- `gcc` (or `build-essential` package on debian distributions)
- `kubectl`
- `curl`
- `rsync`

This project uses [Go Modules](https://github.com/golang/go/wiki/Modules).

## Building and Testing

To setup your environment with the required dependencies, run this at the project root level:
```
make setup
```
This will create a [Kind](https://kind.sigs.k8s.io/) cluster.
Missing system libraries that need installing will be listed in the output

To compile, and create local docker images for each of the sub-projects:
```
make install
```

To run code style checks on all sub-projects:
```
make check-style
```

To run all tests on all sub-projects:
```
make check
```

To run unit tests for the cassandra-operator sub-project:
```
make -C cassandra-operator test
```

### End-to-End Testing

An end-to-end testing approach is used wherever possible.
The end-to-end tests are run in parallel in order to the reduce build time as much as possible.
End-to-end tests are by default run against a local [Kind](https://kind.sigs.k8s.io/) cluster using a [fake-cassandra-docker](fake-cassandra-docker/README.md) image to speed up testing.

To create a [Kind](https://kind.sigs.k8s.io/) Kubernetes cluster:
```
make kind
```

To run all end-to-end tests for the cassandra-operator sub-project:
```
make -C cassandra-operator e2e-test
```

To run a single test suite: 
```
E2E_TEST=modification make -C cassandra-operator e2e-test-parallel
```
 
Additional flags are available to make it possible to run the tests against your own Kubernetes cluster using docker images from your custom repository.
For instance, if you want to run a full build against your cluster with the default `cassandra:3.11` Cassandra image, use this:
```
USE_MOCK=false POD_START_TIMEOUT=5m DOMAIN=mydomain.com KUBE_CONTEXT=k8Context TEST_REGISTRY=myregistry.com/cassandra-operator-test make
```
... where the available flags are:

Flag | Meaning | Default
---|---|---
`USE_MOCK`                     | Whether the Cassandra pods created in the tests should use a `fake-cassandra-docker` image. If true, you can further specify which image to use via the `FAKE_CASSANDRA_IMAGE` flag | `true`
`CASSANDRA_BOOTSTRAPPER_IMAGE` | The fully qualified name for the `cassandra-bootstrapper` docker image | `$(TEST_REGISTRY)/cassandra-bootstrapper:v$(gitRev)`
`FAKE_CASSANDRA_IMAGE`         | The fully qualified name for the `fake-cassandra-docker` docker image | `$(TEST_REGISTRY)/fake-cassandra:v$(gitRev)`
`CASSANDRA_SNAPSHOT_IMAGE`     | The fully qualified name for the `cassandra-snapshot` docker image | `$(TEST_REGISTRY)/cassandra-snapshot:v$(gitRev)`
`CASSANDRA_SIDECAR_IMAGE`      | The fully qualified name for the `cassandra-sidecar` docker image | `$(TEST_REGISTRY)/cassandra-sidecar:v$(gitRev)`
`POD_START_TIMEOUT`            | The max duration allowed for a Cassandra pod to start. The time varies depending on whether a real or fake cassandra image is used and whether PVC or empty dir is used for the cassandra volumes. As a starting point use 150s for fake cassandra otherwise 5m | `150s`
`DOMAIN`                       | Domain name used to create the test operator ingress host | `localhost`
`KUBE_CONTEXT`                 | The Kubernetes context where the test operator will be deployed | `kind`
`KUBECONFIG`                   | The Kubernetes config location where the target `KUBE_CONTEXT` is defined | `$(HOME)/.kube/kind-config-kind`
`TEST_REGISTRY`                | The name of the docker registry where test images created via the build will be pushed| `localhost:5000`
`DOCKER_USERNAME`              | The docker username allowed to push to the release registry | (provided as encrypted variable in `.travis.yml`)
`DOCKER_PASSWORD`              | The password for the docker username allowed to push to the release registry | (provided as encrypted variable in `.travis.yml`)
`GINKGO_COMPILERS`             | Ginkgo `-compilers` value to use when compiling multiple tests suite | `0`, equivalent to not setting the option at all
`GINKGO_NODES`                 | Ginkgo `-nodes` value to use when running tests suite in parallel | `0`, equivalent to not setting the option at all
`E2E_TEST`                     | Name of the end-to-end test suite to run. Use this to run a specific test suite | ``, equivalent to running all test suites


## What to work on

If you want to get involved but are not sure on what issue to pick up,
you should look for an issue with a `good first issue` or `bug` label.

## Pull Request Process

1. If your changes include multiple commits, please squash them into a single commit.  Stack Overflow
   and various blogs can help with this process if you're not already familiar with it.
2. Update the README.md / WIKI where relevant.
3. When submitting your pull request, please provide a comment which describes the change and the problem
   it is intended to resolve. If your pull request is fixing something for which there is a related GitHub issue,
   make reference to that issue with the text "Closes #<issue-number>" in the pull request description.
4. You may merge the pull request to master once a reviewer has approved it. If you do not have permission to
   do that, you may request the reviewer to merge it for you.

## Pinning dependencies

Certain dependencies are picky about using exact combinations of package
versions (in particular, `k8s.io/client-go` and `k8s.io/apimachinery`).
See https://github.com/kubernetes/client-go/blob/master/INSTALL.md#go-modules

The `cassandra-operator/hack/pin-dependency.sh` script is useful for managing
and pinning these versions when the time comes to update them.

For example, to pin code-generator dependencies:

```bash
./hack/pin-dependency.sh k8s.io/code-generator kubernetes-1.15.0-alpha.1
./hack/pin-dependency.sh k8s.io/client-go kubernetes-1.15.0-alpha.1
./hack/pin-dependency.sh k8s.io/apimachinery kubernetes-1.15.0-alpha.1
```

## Releasing

Only maintainers would need to do this:
- Update the CHANGELOG.md with details of all the changes that have been merged since the last release
- Tag the master branch with a new version number; this will trigger a new build which will release
docker images labelled after the tag
- Publish the release (via github), referencing issues and contributors

This project follows the [Semantic Versioning](https://semver.org/) specification, and version numbers
should be chosen accordingly.

## Contributor Code of Conduct

As contributors and maintainers of this project, and in the interest of fostering an open and
welcoming community, we pledge to respect all people who contribute through reporting issues,
posting feature requests, updating documentation, submitting pull requests or patches, and other
activities.

We are committed to making participation in this project a harassment-free experience for everyone,
regardless of level of experience, gender, gender identity and expression, sexual orientation,
disability, personal appearance, body size, race, ethnicity, age, religion, or nationality.

Examples of unacceptable behavior by participants include:

* The use of sexualized language or imagery
* Personal attacks
* Trolling or insulting/derogatory comments
* Public or private harassment
* Publishing other's private information, such as physical or electronic addresses, without explicit
  permission
* Other unethical or unprofessional conduct.

Project maintainers have the right and responsibility to remove, edit, or reject comments, commits,
code, wiki edits, issues, and other contributions that are not aligned to this Code of Conduct. By
adopting this Code of Conduct, project maintainers commit themselves to fairly and consistently
applying these principles to every aspect of managing this project. Project maintainers who do not
follow or enforce the Code of Conduct may be permanently removed from the project team.

This code of conduct applies both within project spaces and in public spaces when an individual is
representing the project or its community.

Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by opening an
issue or contacting one or more of the project maintainers.

This Code of Conduct is adapted from the [Contributor Covenant](http://contributor-covenant.org),
version 1.2.0, available at
[http://contributor-covenant.org/version/1/2/0/](http://contributor-covenant.org/version/1/2/0/)
