GO_CMD=go
GO_TEST=$(GO_CMD) test

COVERAGE_FILE=coverage.txt
COVERHTML_FILE=coverhtml.txt
COVERAGE_SOURCES=$(shell find * -name '*.go' -not -path "testutil/*" -not -path "*testcases/*" | grep -v 'doc.go')

FILES=service/types.proto\
			broadcast/types.proto\
			sbac/types.proto\
			checker/types.proto\
			kv/types.proto

NAMESPACE=chainspace.io
PKG := "./cmd/$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
PROJECT_NAME=chainspace
VERSION := $(shell cat VERSION)

install: $(PROJECT_NAME) httptest httptest2 blockmaniatest pubsublistener ## install the chainspace/httptest binaries

generate: ## generte bindata files # TODO: remove this once new gin-swagger stuff is working
	cd restsrv && go-bindata-assetfs -pkg restsrv -o bindata.go swagger && cd ..

$(PROJECT_NAME): ## build the chainspace binary
	$(GO_CMD) install $(NAMESPACE)/prototype/cmd/$(PROJECT_NAME)

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
	docker build -t $(NAMESPACE)/$(PROJECT_NAME):v$(VERSION) -t gcr.io/acoustic-atom-211511/$(PROJECT_NAME):latest -t gcr.io/acoustic-atom-211511/$(PROJECT_NAME):v$(VERSION) .

docker-push: ## push the docker image to the gcp registry
	docker push gcr.io/acoustic-atom-211511/$(PROJECT_NAME):latest
	docker push gcr.io/acoustic-atom-211511/$(PROJECT_NAME):v$(VERSION)

httptest: ## build the httptest binary
	go install $(NAMESPACE)/prototype/cmd/httptest

httptest2: ## build the httptest2 binary
	go install $(NAMESPACE)/prototype/cmd/httptest2

blockmaniatest: ## build the httptest2 binary
	go install $(NAMESPACE)/prototype/cmd/blockmaniatest

pubsublistener:
	go install $(NAMESPACE)/prototype/cmd/pubsublistener

proto: ## recompile all protobuf definitions
	$(foreach f,$(FILES),\
		./genproto.sh $(f);\
	)

swaggerdocs:
	rm -rf rest/docs && swag init -g rest/router.go && mv docs rest/docs

test: ## Run unit tests
	go test -short ${PKG_LIST} -v

gcp: ## build and compress in order to send to gcp
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" $(NAMESPACE)/prototype/cmd/$(PROJECT_NAME)
	rm -rf ./infrastructure/$(PROJECT_NAME).upx
	upx -o ./infrastructure/$(PROJECT_NAME).upx $(PROJECT_NAME)

contract: ## build dummy contract docker
	docker build -t "$(NAMESPACE)/contract-dummy:latest" -t "gcr.io/acoustic-atom-211511/$(NAMESPACE)/contract-dummy:latest" -f ./dummycontract/Dockerfile ./dummycontract

.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
