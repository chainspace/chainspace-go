FILES=	service/types.proto\
	broadcast/types.proto\
	transactor/types.proto

install: ## install the chainspace binary
	go install chainspace.io/prototype/cmd/chainspace

proto: ## recompile all protobuf definitions
	$(foreach f,$(FILES),\
		./genproto.sh $(f);\
	)

.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.SILENT:
