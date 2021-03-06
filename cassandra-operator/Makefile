.DEFAULT_GOAL := all
.PHONY: all clean install check build fmt vet lint validate-licences test e2e-te setup go-build go-install docker-image push-image release check-style \
	validate-crds generate-crds validate-codegen generate-codegen generate-all

# Default context is running locally against a kind cluster
KUBECONFIG ?= $(HOME)/.kube/config
KUBE_CONTEXT ?= kind
POD_START_TIMEOUT ?= 150s
USE_MOCK ?= true
FAKE_CASSANDRA_IMAGE ?= $(TEST_REGISTRY)/fake-cassandra:v$(gitRev)
CASSANDRA_BOOTSTRAPPER_IMAGE ?= $(TEST_REGISTRY)/cassandra-bootstrapper:v$(gitRev)
CASSANDRA_SIDECAR_IMAGE ?= $(TEST_REGISTRY)/cassandra-sidecar:v$(gitRev)
CASSANDRA_SNAPSHOT_IMAGE ?= $(TEST_REGISTRY)/cassandra-snapshot:v$(gitRev)
DOMAIN ?= localhost
IMAGE_TO_DEPLOY ?= $(dockerTestImage)
NAMESPACE ?= test-cassandra-operator
GINKGO_PARALLEL_OPTIONS ?= $(if $(GINKGO_NODES),-p -nodes=$(GINKGO_NODES),)
GINKGO_COMPILERS ?= 0
E2E_TEST ?=
SKIP_PACKAGES ?=
GOOS ?= linux
GOARCH ?= amd64

projectDir := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))
buildDir := $(projectDir)/build
image := cassandra-operator
junitReportDir := $(buildDir)/junit-reports
pkgs := $(shell go list ./... | grep -v /vendor/ | grep -v /pkg/apis/ | grep -v /pkg/client/ | grep -v /test/)
testPkgs := $(shell go list ./... | grep /test/e2e)

include $(projectDir)/../MakefileDocker

all: clean install check

build: go-build check-style validate-licences validate-crds validate-codegen docker-image

clean-test-namespace:
	@echo "== delete all in test-cassandra-operator"
	kubectl --context ${KUBE_CONTEXT} -n test-cassandra-operator delete all --all
	kubectl --context ${KUBE_CONTEXT} -n test-cassandra-operator delete cassandras --all

install: build go-install push-image

clean-redeploy-test: clean-test-namespace install deploy-operator

check-style: fmt vet lint

check: test deploy-operator integration-test e2e-test

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
		golang.org/x/tools/cmd/goimports \
		golang.org/x/lint/golint \
		sigs.k8s.io/controller-tools/cmd/controller-gen

go-build: ensure-build-dir-exists ensure-go-modules
	@echo "== build"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(buildDir)/bin/cassandra-operator -v github.com/sky-uk/cassandra-operator/cassandra-operator/cmd/operator
	KUBE_CONTEXT=ignore go test -run xxxxx $(testPkgs) # build the test code but don't run any tests yet

go-install: go-build
	@echo "== install"
	cp -v $(buildDir)/bin/cassandra-operator $(shell go env GOPATH)/bin/cassandra-operator

fmt:
ifneq (, $(shell gofmt -l .))
	$(error "gofmt found formatting issues: $(shell gofmt -l .). You may want to run `go fmt ./...` from the module folder")
endif

vet:
	go vet $(pkgs)

lint:
	for pkg in $(pkgs); do \
		golint -set_exit_status $$pkg || exit 1; \
	done;

ensure-build-dir-exists:
	mkdir -p $(buildDir)

ensure-test-report-dir-exists: ensure-build-dir-exists
	mkdir -p $(junitReportDir)

test: ensure-test-report-dir-exists
	@echo "== test"
	ginkgo -r -skipPackage=$(SKIP_PACKAGES) -compilers=$(GINKGO_COMPILERS) --v --progress pkg cmd -- -junit-report-dir $(junitReportDir)

integration-test: ensure-test-report-dir-exists
	@echo "== integration-test"
ifeq ($(strip $(USE_MOCK)),true)
	@echo "== e2e-test integration test against real cassandra"
	KUBE_CONTEXT=$(KUBE_CONTEXT) \
		KUBECONFIG=$(KUBECONFIG) \
		NAMESPACE=$(NAMESPACE) \
		POD_START_TIMEOUT=5m \
		USE_MOCK=false \
		CASSANDRA_SNAPSHOT_IMAGE=$(CASSANDRA_SNAPSHOT_IMAGE) \
		CASSANDRA_BOOTSTRAPPER_IMAGE=$(CASSANDRA_BOOTSTRAPPER_IMAGE) \
		CASSANDRA_SIDECAR_IMAGE=$(CASSANDRA_SIDECAR_IMAGE) \
		ginkgo $(GINKGO_PARALLEL_OPTIONS) -r -skipPackage=$(SKIP_PACKAGES) -compilers=$(GINKGO_COMPILERS) --v --progress -stream -timeout=30m test/e2e/parallel/creation -- -junit-report-dir $(junitReportDir)
endif

e2e-test: e2e-test-parallel e2e-test-sequential

e2e-test-parallel: ensure-test-report-dir-exists
	@echo "== e2e-test-parallel"
	KUBE_CONTEXT=$(KUBE_CONTEXT) \
		KUBECONFIG=$(KUBECONFIG) \
		NAMESPACE=$(NAMESPACE) \
		POD_START_TIMEOUT=$(POD_START_TIMEOUT) \
		FAKE_CASSANDRA_IMAGE=$(FAKE_CASSANDRA_IMAGE) \
		USE_MOCK=$(USE_MOCK) \
		CASSANDRA_SNAPSHOT_IMAGE=$(CASSANDRA_SNAPSHOT_IMAGE) \
		CASSANDRA_BOOTSTRAPPER_IMAGE=$(CASSANDRA_BOOTSTRAPPER_IMAGE) \
		CASSANDRA_SIDECAR_IMAGE=$(CASSANDRA_SIDECAR_IMAGE) \
		ginkgo $(GINKGO_PARALLEL_OPTIONS) -r -skipPackage=$(SKIP_PACKAGES) -compilers=$(GINKGO_COMPILERS) --v --progress -stream -timeout=30m test/e2e/parallel/$(E2E_TEST) -- -junit-report-dir $(junitReportDir)

e2e-test-sequential: ensure-test-report-dir-exists
	@echo "== e2e-test-sequential"
	KUBE_CONTEXT=$(KUBE_CONTEXT) \
		KUBECONFIG=$(KUBECONFIG) \
		NAMESPACE=$(NAMESPACE) \
		POD_START_TIMEOUT=$(POD_START_TIMEOUT) \
		FAKE_CASSANDRA_IMAGE=$(FAKE_CASSANDRA_IMAGE) \
		USE_MOCK=$(USE_MOCK) \
		CASSANDRA_SNAPSHOT_IMAGE=$(CASSANDRA_SNAPSHOT_IMAGE) \
		CASSANDRA_BOOTSTRAPPER_IMAGE=$(CASSANDRA_BOOTSTRAPPER_IMAGE) \
		CASSANDRA_SIDECAR_IMAGE=$(CASSANDRA_SIDECAR_IMAGE) \
		ginkgo -r -skipPackage=$(SKIP_PACKAGES) -compilers=$(GINKGO_COMPILERS) --v --progress -timeout=30m test/e2e/sequential/$(E2E_TEST) -- -junit-report-dir $(junitReportDir)

docker-image: go-build common-docker-build

push-image: docker-image common-docker-push

deploy-operator:
	@echo "== deploy-operator"
	IMAGE=$(IMAGE_TO_DEPLOY) KUBECONFIG=$(KUBECONFIG) CONTEXT=$(KUBE_CONTEXT) NAMESPACE=$(NAMESPACE) INGRESS_HOST=$(NAMESPACE).cassandra-operator.$(DOMAIN) WITH_CRD="true" $(projectDir)/hack/deploy.sh

deploy-operator-without-crd:
	@echo "== deploy-operator-without-crd"
	IMAGE=$(IMAGE_TO_DEPLOY) KUBECONFIG=$(KUBECONFIG) CONTEXT=$(KUBE_CONTEXT) NAMESPACE=$(NAMESPACE) INGRESS_HOST=$(NAMESPACE).cassandra-operator.$(DOMAIN) WITH_CRD="false" $(projectDir)/hack/deploy.sh

validate-licences:
	@echo "== validate licences"
	set -e ;\
	restricted=$$(paste -s -d ',' restricted-licences.txt) ;\
	licence-compliance-checker -L error -E -r $$restricted \
		-m github.com/sky-uk/cassandra-operator/cassandra-operator=BSD-3-Clause \
		-m sigs.k8s.io/yaml=BSD-3-Clause \
		--check-go-modules ;\

clean: common-docker-clean
	@echo "== clean"
	rm $(shell go env GOPATH)/bin/cassandra-operator || true
	rm -rfv $(buildDir)

generate-all: generate-codegen generate-crds

generate-codegen: setup
	@echo "== generate-codegen"
	$(projectDir)/hack/update-codegen.sh

validate-codegen: setup
	@echo "== validate-codegen"
	$(projectDir)/hack/verify-codegen.sh

generate-crds: setup
	@echo "== generate-crds"
	$(projectDir)/hack/update-crds.sh

validate-crds: setup
	@echo "== validate-crds"
	$(projectDir)/hack/verify-crds.sh

release: common-docker-release
