LIBRARIES = $(patsubst %/go.mod, %, $(wildcard */go.mod))
TEST_LIBRARIES = $(addprefix test-,$(LIBRARIES))
GOMOD_LIBRARIES = $(addprefix gomod-,$(LIBRARIES))

help:
	@echo "# Makefile Help #"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'


$(TEST_LIBRARIES): test-%:
	$(MAKE) -C "$*"

$(GOMOD_LIBRARIES): gomod-%:
	cd "$*" && go mod tidy

test: $(TEST_LIBRARIES) ## Test all libraries
	@echo "All tested"

release: ## Release all libraries
	go run ./utils/release $(LIBRARIES) $(version)
	@echo "All released"

gomod: $(GOMOD_LIBRARIES) ## go mod tidy all libraries
	@echo "All go mod tidied"

release-util-test:
	go test ./utils/release
