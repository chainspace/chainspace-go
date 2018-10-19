GO_CMD=go
GO_TEST=$(GO_CMD) test

COVERAGE_FILE=coverage.txt
COVERHTML_FILE=coverhtml.txt
COVERAGE_SOURCES=$(shell find * -name '*.go' -not -path "testutil/*" -not -path "*testcases/*" | grep -v 'doc.go')

FILES=	service/types.proto\
	broadcast/types.proto\
	transactor/types.proto\
	kv/types.proto

PKG := "./cmd/$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
PROJECT_NAME := "chainspace"

install: chainspace httptest ## install the chainspace/httptest binaries

chainspace: ## build the chainspace binary
	$(GO_CMD) install chainspace.io/prototype/cmd/chainspace

coverage: $(COVERAGE_FILE)

$(COVERAGE_FILE): $(COVERAGE_SOURCES)
	@for d in $(PKG_LIST); do \
	    $(GO_TEST) -coverprofile=profile.out -covermode=atomic $$d || exit 1; \
	    if [ -f profile.out ]; then \
	        cat profile.out >> $(COVERAGE_FILE); \
	        rm profile.out; \
	    fi \
		done

coverhtml:
	echo 'mode: set' > $(COVERHTML_FILE)
	@for d in $(PKG_LIST); do \
	    $(GO_TEST) -coverprofile=profile.out $$d || exit 1; \
	    if [ -f profile.out ]; then \
	        tail -n +2 profile.out >> $(COVERHTML_FILE); \
	        rm profile.out; \
	    fi \
	done
	$(GO_CMD) tool cover -html $(COVERHTML_FILE)

docker-all: docker docker-push ## build the docker image and push it to the gcp registry

docker: ## build the docker image
	docker build -t chainspace.io/chainspace:v0.1 -t gcr.io/acoustic-atom-211511/chainspace:latest -t gcr.io/acoustic-atom-211511/chainspace:v0.1 .

docker-push: ## push the docker image to the gcp registry
	docker push gcr.io/acoustic-atom-211511/chainspace:latest
	docker push gcr.io/acoustic-atom-211511/chainspace:v0.1

httptest: ## build the httptest binary
	go install chainspace.io/prototype/cmd/httptest

proto: ## recompile all protobuf definitions
	$(foreach f,$(FILES),\
		./genproto.sh $(f);\
	)

test: ## Run unit tests
	go test -short ${PKG_LIST} -v

gcp: ## build and compress in order to send to gcp
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" chainspace.io/prototype/cmd/chainspace
	rm -rf ./infrastructure/chainspace.upx
	upx -o ./infrastructure/chainspace.upx chainspace

contract: ## build dummy contract docker
	docker build -t "chainspace.io/contract-dummy:latest" -t "gcr.io/acoustic-atom-211511/chainspace.io/contract-dummy:latest" -f ./dummycontract/Dockerfile ./dummycontract

.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
