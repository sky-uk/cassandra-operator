.DEFAULT_GOAL := all
.PHONY: all clean install check build fmt vet lint validate-licences test e2e-te setup go-build go-install docker-image push-image prepare-deployment release check-style

# Default context is running locally against a kind cluster
KUBECONFIG ?= $(HOME)/.kube/kind-config-kind
KUBE_CONTEXT ?= kind
USE_MOCK ?= true
FAKE_CASSANDRA_IMAGE ?= $(TEST_REGISTRY)/fake-cassandra:v$(gitRev)
IMAGE_UNDER_TEST ?= $(dockerTestImage)
NAMESPACE ?= test-cassandra-operator
GINKGO_NODES ?= 0
GINKGO_COMPILERS ?= 0
E2E_TEST ?=

projectDir := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))
k8ResourcesDir := $(projectDir)/../cassandra-operator/kubernetes-resources
buildDir := $(projectDir)/build
image := cassandra-snapshot
junitReportDir = $(buildDir)/junit-reports
pkgs := $(shell go list ./... | grep -v /vendor/ | grep -v /test/e2e)
testPkgs := $(shell go list ./... | grep /test/e2e)

include $(projectDir)/../MakefileDocker

all: clean install check

build: go-build check-style validate-licences docker-image

install: build go-install push-image

check-style: fmt vet lint

check: test prepare-deployment integration-test e2e-test

ensure-go-modules:
ifeq ('$(shell go env | grep -o "GOMOD.*")', 'GOMOD=""')
	$(error "go modules must be enabled: export GO111MODULE=on, or run from outside $$GOPATH")
else
	@echo "go modules enabled"
endif

setup: ensure-go-modules
	@echo "== setup"
	go install \
		github.com/sky-uk/licence-compliance-checker \
		github.com/onsi/ginkgo/ginkgo \
		golang.org/x/lint/golint

go-build: ensure-build-dir-exists ensure-go-modules
	@echo "== build"
	go build -o $(buildDir)/bin/cassandra-snapshot -v github.com/sky-uk/cassandra-operator/cassandra-snapshot/cmd
	KUBE_CONTEXT=ignore go test -run xxxxx $(testPkgs)  # build the test code but don't run any tests yet

go-install:
	@echo "== go-install"
	cp -v $(buildDir)/bin/cassandra-snapshot $(shell go env GOPATH)/bin/cassandra-snapshot

fmt:
	go fmt ./...

validate-licences:
	@echo "== validate licences"
	set -e ;\
	restricted=$$(paste -s -d ',' restricted-licences.txt) ;\
	licence-compliance-checker -L error -E -r $$restricted \
		-m github.com/sky-uk/cassandra-operator/cassandra-snapshot=BSD-3-Clause \
		-m github.com/sky-uk/cassandra-operator/cassandra-operator=BSD-3-Clause \
		-m sigs.k8s.io/yaml=BSD-3-Clause \
		--check-go-modules ;\

vet:
	@echo "== vet"
	go vet $(pkgs)

lint:
	@echo "== lint"
	for pkg in $(pkgs); do \
		golint -set_exit_status $$pkg || exit 1; \
	done;

ensure-build-dir-exists:
	mkdir -p $(buildDir)

ensure-test-report-dir-exists: ensure-build-dir-exists
	mkdir -p $(junitReportDir)

test: ensure-test-report-dir-exists
	@echo "== test"
	ginkgo -p -nodes=$(GINKGO_NODES) -r -compilers=$(GINKGO_COMPILERS) --v --progress pkg cmd -- -junit-report-dir $(junitReportDir)

integration-test: ensure-test-report-dir-exists
	@echo "== integration-test"
ifeq ($(strip $(USE_MOCK)),true)
	@echo "== e2e-test integration test against real cassandra"
	IMAGE_UNDER_TEST=$(dockerTestImage) \
				KUBE_CONTEXT=$(KUBE_CONTEXT) \
				KUBECONFIG=$(KUBECONFIG) \
				NAMESPACE=$(NAMESPACE) \
				USE_MOCK=false \
				ginkgo -r -compilers=$(GINKGO_COMPILERS) --v --progress test/e2e/create -- -junit-report-dir $(junitReportDir)
endif

e2e-test: ensure-test-report-dir-exists
	@echo "== e2e-test"
	IMAGE_UNDER_TEST=$(IMAGE_UNDER_TEST) \
				FAKE_CASSANDRA_IMAGE=$(FAKE_CASSANDRA_IMAGE) \
				KUBE_CONTEXT=$(KUBE_CONTEXT) \
				KUBECONFIG=$(KUBECONFIG) \
				NAMESPACE=$(NAMESPACE) \
				USE_MOCK=$(USE_MOCK) \
				ginkgo -r -compilers=$(GINKGO_COMPILERS) --v --progress test/e2e/$(E2E_TEST) -- -junit-report-dir $(junitReportDir)

docker-image: go-build common-docker-build

push-image: docker-image common-docker-push

prepare-deployment:
	@echo "== prepare-deployment"
	sed -e "s@\$$TARGET_NAMESPACE@$(NAMESPACE)@g" $(k8ResourcesDir)/cassandra-snapshot.yml > ${buildDir}/cassandra-snapshot.yml
	kubectl --kubeconfig $(KUBECONFIG) --context $(KUBE_CONTEXT) -n $(NAMESPACE) apply -f ${buildDir}/cassandra-snapshot.yml

clean: common-docker-clean
	@echo "== clean"
	rm $(shell go env GOPATH)/bin/cassandra-snapshot || true
	rm -rfv $(buildDir)

release: common-docker-release
