.DEFAULT_GOAL := all
.PHONY: all clean setup build install jar check dgoss docker-image push-image

KUBE_CONTEXT ?= dind
NAMESPACE ?= test-cassandra-operator
CASSANDRA_BOOTSTRAPPER_IMAGE ?= $(TEST_REGISTRY)/cassandra-bootstrapper:v$(gitRev)
CASSANDRA_SIDECAR_IMAGE ?= $(TEST_REGISTRY)/cassandra-sidecar:v$(gitRev)
NFT_IMAGE ?= $(dockerTestImage)

projectDir := $(realpath $(dir $(firstword $(MAKEFILE_LIST))))
gradle := $(projectDir)/../gradlew
image := nft

include $(projectDir)/../MakefileDocker

all: clean install check

build: check-style jar docker-image

install: build push-image

setup:
	@echo "== setup not implemented"

check-style :
	@echo "== check-style not implemented"

docker-image: common-docker-build

push-image: docker-image common-docker-push

nft-test:
	@echo "== nft-test"
	CONTEXT=$(KUBE_CONTEXT) \
	    NAMESPACE=$(NAMESPACE) \
	    NFT_IMAGE=$(NFT_IMAGE) \
	    CASSANDRA_BOOTSTRAPPER_IMAGE=$(CASSANDRA_BOOTSTRAPPER_IMAGE) \
	    CASSANDRA_SIDECAR_IMAGE=$(CASSANDRA_SIDECAR_IMAGE) \
	    $(projectDir)/hack/run-nft.sh
