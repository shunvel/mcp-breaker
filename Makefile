GO ?= go
BINARY := mcp-breaker

.PHONY: build test vet clean demo validate validate-all evidence negative-tests dev-ui test-ui-e2e

build:
	$(GO) build -o $(BINARY) ./cmd/mcp-breaker

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

demo:
	bash scripts/demo.sh

validate:
	bash scripts/validate.sh

validate-all: validate negative-tests test-ui-e2e

evidence:
	bash scripts/validate.sh
	python3 scripts/render_evidence.py

negative-tests:
	bash scripts/negative_tests.sh

dev-ui:
	bash scripts/dev_ui.sh

test-ui-e2e:
	export PATH="$${HOME}/sdk/go/bin:$$PATH"; \
	$(MAKE) build; \
	go build -o fakemcp ./internal/testmcp/cmd/fakemcp; \
	python3 scripts/test_ui_e2e.py

clean:
	rm -f $(BINARY) fakemcp demo-stdout.ndjson demo-stderr.log validate-stdout.ndjson validate-graph.ndjson
