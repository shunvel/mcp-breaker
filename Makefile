GO ?= go
BINARY := mcp-breaker

.PHONY: build test vet clean demo

build:
	$(GO) build -o $(BINARY) ./cmd/mcp-breaker

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

demo:
	bash scripts/demo.sh

clean:
	rm -f $(BINARY) fakemcp demo-stdout.ndjson demo-stderr.log
