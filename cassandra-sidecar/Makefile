.DEFAULT_GOAL := travis
.PHONY: travis clean install build fmt vet lint validate-licences setup go-build go-install docker-image push-image release check-style

GOOS ?= linux
GOARCH ?= amd64

projectDir := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))
buildDir := $(projectDir)/build
image := cassandra-sidecar
pkgs := $(shell go list ./... | grep -v /vendor/ | grep -v /test/e2e)

include $(projectDir)/../MakefileDocker

travis: clean install

build: go-build check-style validate-licences docker-image

install: build go-install push-image

check:
	@echo "== check not implemented"

check-style: fmt vet lint

ensure-go-modules:
ifeq ('$(shell go env | grep -o "GOMOD.*")', 'GOMOD=""')
	$(error "go modules must be enabled: export GO111MODULE=on, or run from outside $$GOPATH")
else
	@echo "go modules enabled"
endif

setup:
	@echo "== setup"
	go install \
		github.com/sky-uk/licence-compliance-checker \
		github.com/onsi/ginkgo/ginkgo \
		golang.org/x/tools/cmd/goimports \
		golang.org/x/lint/golint

go-build: ensure-build-dir-exists ensure-go-modules
	@echo "== build"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(buildDir)/bin/cassandra-sidecar -v github.com/sky-uk/cassandra-operator/cassandra-sidecar/cmd

go-install:
	@echo "== go-install"
	cp -v $(buildDir)/bin/cassandra-sidecar $(shell go env GOPATH)/bin/cassandra-sidecar

fmt:
ifneq (, $(shell gofmt -l .))
	$(error "gofmt found formatting issues: $(shell gofmt -l .). You may want to run `go fmt ./...` from the module folder")
endif

validate-licences:
	@echo "== validate licences"
	set -e ;\
	restricted=$$(paste -s -d ',' restricted-licences.txt) ;\
	licence-compliance-checker -L error -E -r $$restricted \
		-m github.com/sky-uk/cassandra-operator/cassandra-sidecar=BSD-3-Clause \
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

docker-image: go-build common-docker-build

push-image: docker-image common-docker-push

clean: common-docker-clean
	@echo "== clean"
	rm $(shell go env GOPATH)/bin/cassandra-sidecar || true
	rm -rfv $(buildDir)

release: common-docker-release
