GO ?= go
BINARY := mcp-breaker

.PHONY: build test vet clean demo validate evidence negative-tests

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

evidence:
	bash scripts/validate.sh
	python3 scripts/render_evidence.py

negative-tests:
	bash scripts/negative_tests.sh

clean:
	rm -f $(BINARY) fakemcp demo-stdout.ndjson demo-stderr.log validate-stdout.ndjson validate-graph.ndjson
