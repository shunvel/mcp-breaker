# Contributing to mcp-breaker

Thank you for your interest in contributing to [mcp-breaker](https://github.com/shunvel/mcp-breaker). This project aims to stop AI agents from spinning in semantic tool loops — every contribution helps make agent workflows more reliable.

## Ways to contribute

- **Report bugs** — open an issue with reproduction steps
- **Suggest features** — describe the problem and proposed behavior
- **Submit pull requests** — bug fixes, tests, docs, and new detection modules
- **Improve documentation** — README, examples, client integration guides

## Development setup

### Prerequisites

- Go **1.23+** — [go.dev/dl](https://go.dev/dl/)
- Git

### Clone and build

```bash
git clone https://github.com/shunvel/mcp-breaker.git
cd mcp-breaker
make build
```

If Go is on your `PATH`, you can override the Makefile default:

```bash
GO=go make build
```

### Run tests locally

```bash
make test    # full test suite
make vet     # static analysis
make demo    # local echo-breaker demo (no MCP config required)
```

Equivalent direct commands:

```bash
go test ./... -count=1
go vet ./...
```

## Project conventions

### Code style

- Follow standard Go conventions (`gofmt`, idiomatic naming)
- Keep changes focused — one logical change per pull request
- Prefer stdlib over new dependencies unless justified in the PR description
- Add tests for new behavior; do not reduce existing coverage

### Architecture boundaries

| Package | Responsibility |
|---------|----------------|
| `pkg/proxy` | JSON-RPC framing, session proxy, stdio wrap — **no breaker logic** |
| `pkg/breaker` | Detection modules (echo, semantic, etc.) implementing `proxy.Interceptor` |
| `pkg/protocol` | MCP types and intervention payload helpers |
| `cmd/mcp-breaker` | CLI entry point only |
| `internal/testmcp` | Fake MCP server for tests and demos |

Avoid import cycles: `pkg/proxy` must not import `pkg/breaker`.

### Commit messages

Write clear, concise commit messages focused on **why** the change was made:

```
Add cosine similarity stub for semantic detector

Prepares the interceptor interface for ONNX embedding integration
without changing proxy behavior.
```

## Pull request process

1. **Fork** the repository and create a feature branch from `main`
2. **Make your changes** with tests where applicable
3. **Run the checks** locally:
   ```bash
   make test && make vet
   ```
4. **Push** your branch and open a pull request against `main`
5. **Fill in the PR template** (if present) or describe:
   - What changed and why
   - How you tested it
   - Any breaking changes

CI must pass before merge. The workflow runs `go vet`, `go test -race`, build, and a demo smoke test.

### Review expectations

- Maintainers may request changes or suggest alternatives
- Keep PRs small and reviewable when possible
- Be respectful and constructive in discussions

## Reporting bugs

Open a [GitHub Issue](https://github.com/shunvel/mcp-breaker/issues/new) and include:

1. **Environment** — OS, Go version, AI client (Cursor, Claude Desktop, etc.)
2. **MCP server** — command being wrapped (e.g. `npx @modelcontextprotocol/server-filesystem`)
3. **Steps to reproduce** — config snippet, tool calls, or `make demo` output
4. **Expected vs actual behavior**
5. **Logs** — proxy stderr output (redact secrets)

## Feature requests

For new detection modules (semantic similarity, graph loops, dashboard, etc.), refer to [spec.md](spec.md) and open an issue describing:

- Which spec section it addresses
- Proposed algorithm and intervention behavior
- Performance constraints (latency budget, memory footprint)

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

---

Questions? Open an issue or start a discussion on GitHub.
