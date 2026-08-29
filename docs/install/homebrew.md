# Homebrew installation

Install **mcp-breaker** via a Homebrew tap (formula ships in this repository under [`Formula/mcp-breaker.rb`](../../Formula/mcp-breaker.rb)).

## Tap setup (maintainers)

Create a public tap repository named **`homebrew-mcp-breaker`** under the `shunvel` GitHub account, then copy the formula:

```bash
# One-time tap repo setup
gh repo create shunvel/homebrew-mcp-breaker --public
cp Formula/mcp-breaker.rb /path/to/homebrew-mcp-breaker/Formula/mcp-breaker.rb
```

Users install with:

```bash
brew tap shunvel/mcp-breaker
brew install mcp-breaker onnxruntime
mcp-breaker models download
```

## Verify

```bash
mcp-breaker help
mcp-breaker models status
make demo   # from source checkout
```

## ONNX Runtime

The semantic stagnation tracker requires:

1. **onnxruntime** — installed via Homebrew (`brew install onnxruntime`)
2. **Model cache** — `mcp-breaker models download` (~25MB into `~/.cache/mcp-breaker/models/`)

Set the library path if auto-detection fails:

```bash
export ONNXRUNTIME_LIB_PATH="$(brew --prefix onnxruntime)/lib/libonnxruntime.dylib"
```

## Development without ONNX

For local development and tests without ONNX Runtime:

```bash
export MCP_BREAKER_MOCK_EMBED=1
mcp-breaker wrap -- ./fakemcp
```

This enables the mock embedder for semantic stagnation detection (Test Case B logic).

## Build from source (no tap)

```bash
git clone https://github.com/shunvel/mcp-breaker.git
cd mcp-breaker
make build
./mcp-breaker models download
```

Optional ONNX-enabled build:

```bash
go build -tags=onnx -o mcp-breaker ./cmd/mcp-breaker
```
