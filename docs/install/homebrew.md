# Homebrew installation

Install **mcp-breaker** from the formula in this repository (`Formula/mcp-breaker.rb`). GoReleaser updates that formula automatically on each release.

## Install

```bash
brew tap shunvel/mcp-breaker https://github.com/shunvel/mcp-breaker
brew install mcp-breaker
mcp-breaker models download
```

The tap URL points at this repo (not a separate `homebrew-mcp-breaker` repository). GoReleaser commits formula updates to `Formula/mcp-breaker.rb` when a version tag is pushed.

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

## Build from source (no Homebrew)

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

## Prebuilt binaries

If you prefer not to use Homebrew, see [releases.md](releases.md) for GitHub Release downloads on macOS, Linux, and Windows.
