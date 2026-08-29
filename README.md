# mcp-breaker

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CI](https://github.com/shunvel/mcp-breaker/actions/workflows/ci.yml/badge.svg)](https://github.com/shunvel/mcp-breaker/actions/workflows/ci.yml)

**Semantic MCP Circuit Breaker** — an open-source, zero-dependency JSON-RPC proxy that sits between AI clients and [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers. It detects tool invocation loops early and intervenes gracefully before they drain API credits and flood agent context windows.

> Repository: [github.com/shunvel/mcp-breaker](https://github.com/shunvel/mcp-breaker)

---

## Table of Contents

- [Problem Statement](#problem-statement)
- [Solution](#solution)
- [Architecture](#architecture)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [Client Configuration](#client-configuration)
- [How the Echo Breaker Works](#how-the-echo-breaker-works)
- [Project Structure](#project-structure)
- [Development](#development)
- [Roadmap](#roadmap)
- [License](#license)

---

## Problem Statement

Autonomous AI agents using MCP frequently hit **semantic stagnation loops**. Unlike traditional code loops that crash or throw exceptions, an agent logic loop returns successful responses on every turn. The agent appears to run correctly from an engineering standpoint, but its reasoning is locked in a recursive pattern:

- Executing the same terminal command repeatedly (`npm run test`, `go build`, …)
- Rewriting files with synonymous or cosmetic changes
- Calling identical tool parameters with no new outcomes

### Why existing guardrails fail

| Pain point | Description |
|------------|-------------|
| **Late triggers** | Numeric limits (`max_turns = 20`) fire too late. By then, API credits are spent and the context window is full of repetitive logs. |
| **Context degradation** | Short-term memory fills with low-signal responses, preventing the agent from breaking free on its own. |
| **Hard aborts** | Many tools terminate the session entirely instead of redirecting the agent toward a different strategy. |

**mcp-breaker** addresses this by intercepting MCP traffic at the transport layer, flagging loops within **2–3 iterations**, and returning structured interventions without killing the active chat session.

---

## Solution

mcp-breaker runs as a transparent stdio proxy between your AI client (Cursor, Claude Desktop, Cline, etc.) and any MCP server. It monitors `tools/call` JSON-RPC frames, detects repetitive invocations, and either forwards the request or returns a synthetic intervention response.

```
+-----------+    JSON-RPC (stdio)      +-----------------+    Proxied Call    +------------+
| AI Client | ───────────────────────> |   mcp-breaker   | ─────────────────> | MCP Server |
|  (Cursor) | <─────────────────────── |  (Proxy Layer)  | <───────────────── |  (Target)  |
+-----------+    System Intervention   +-----------------+    Tool Response   +------------+
                          │
                          ▼
                 +-----------------+
                 |  Echo Detector  |
                 |  (hash ring)    |
                 +-----------------+
```

No MCP server needs to be running beforehand. The proxy **launches the child server** on each session via `wrap`.

Wire format follows the official [MCP stdio transport specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio): one JSON-RPC object per line, logs on stderr only.

---

## Features

### Implemented

| Module | Description |
|--------|-------------|
| **Stdio JSON-RPC proxy** | Newline-delimited framing, bidirectional streaming, malformed frame recovery |
| **Tool echo detector** | Per-tool hash ring (last 5 calls); trips on the 3rd consecutive identical `tools/call` |
| **Semantic stagnation tracker** | Embedding cosine similarity (N vs N−2) ≥ 0.88 on tool results; spec Test Case B |
| **Sliding-window graph ledger** | Detects A→B→A→B loops; blocks and pauses until dashboard resume |
| **TUI dashboard** | `mcp-breaker dashboard` — live stream, metrics, cosine scores, resume control |
| **Model management** | `mcp-breaker models download|status` for All-MiniLM-L6-v2 ONNX cache |
| **Homebrew formula** | [`Formula/mcp-breaker.rb`](Formula/mcp-breaker.rb) — see [install guide](docs/install/homebrew.md) |
| **Graceful intervention** | Returns MCP `isError` results instead of crashing the session |
| **Built-in test server** | `fakemcp` for local demos with zero external dependencies |

### Planned

| Module | Description |
|--------|-------------|
| **Config auto-wrap (`init`)** | Rewrites Cursor / Claude Desktop MCP configs automatically |
| **Full ONNX embed pipeline** | `-tags=onnx` build with in-process All-MiniLM inference (stub today) |

See [spec.md](spec.md) for the full product specification.

---

## Requirements

- **Go 1.24+** — [go.dev/dl](https://go.dev/dl/)
- **macOS / Linux** — stdio proxy (Windows support planned)
- **Optional:** Node.js + `npx` — only if wrapping npm-based MCP servers
- **Optional:** `onnxruntime` + `mcp-breaker models download` — for semantic tracker (or `MCP_BREAKER_MOCK_EMBED=1` for dev)

---

## Installation

### From source

```bash
git clone https://github.com/shunvel/mcp-breaker.git
cd mcp-breaker
make build
```

This produces a single static binary at `./mcp-breaker`.

### Homebrew

```bash
brew tap shunvel/mcp-breaker   # after tap repo is published
brew install mcp-breaker onnxruntime
mcp-breaker models download
```

See [docs/install/homebrew.md](docs/install/homebrew.md) for tap setup and ONNX configuration.

### Verify the build

```bash
make test    # run the full test suite
make validate # run all core flows: tests, echo, semantic, graph ledger
make vet     # static analysis
./mcp-breaker help
```

---

## Quick Start

### Option 1 — Local demo (no MCP config required)

The repo ships with a fake MCP server for testing. No Cursor setup, no running servers:

```bash
make demo
```

Expected output:

```
id=1: initialize -> testmcp
id=2: [forwarded] ok
id=3: [forwarded] ok
id=4: [BLOCKED] Error: Command [write_file] generated identical failures...
```

### Option 2 — Wrap a real MCP server

```bash
./mcp-breaker wrap -- npx -y @modelcontextprotocol/server-filesystem /tmp
```

The proxy starts the child process, forwards stdin/stdout, and monitors every `tools/call`.

### Option 3 — Wrap the built-in test server

```bash
go build -o fakemcp ./internal/testmcp/cmd/fakemcp
./mcp-breaker wrap -- ./fakemcp
```

### Option 4 — Dashboard + wrap

```bash
# Terminal 1
./mcp-breaker wrap -- ./fakemcp

# Terminal 2
./mcp-breaker dashboard
```

Press **R** in the dashboard to resume after a graph loop pause.

---

## CLI Reference

```
mcp-breaker — semantic MCP circuit breaker proxy

Usage:
  mcp-breaker wrap [flags] -- <command> [args...]
  mcp-breaker dashboard
  mcp-breaker models download|status
  mcp-breaker help

Commands:
  wrap       Launch child MCP server through the circuit breaker stack
  dashboard  Terminal UI for live metrics and loop alerts
  models     Download or inspect ONNX embedding model cache
  help       Show usage information
```

### `wrap`

```bash
mcp-breaker wrap [--no-semantic] [--semantic-threshold=0.88] -- <command> [args...]
```

| Flag / argument | Description |
|-----------------|-------------|
| `--` | Required separator. Everything after `--` is passed to the child process verbatim. |
| `--no-semantic` | Disable semantic stagnation detector |
| `--semantic-threshold` | Cosine similarity trip threshold (default 0.88) |
| `<command>` | The MCP server executable (e.g. `npx`, `node`, `./fakemcp`). |
| `[args...]` | Arguments for the child server. |

**Behavior:**

- Reads JSON-RPC frames from **stdin** (the AI client), writes responses to **stdout**
- Forwards child **stderr** to the proxy's stderr (never stdout — that would break framing)
- Closes the child when the client disconnects
- Handles `SIGINT` / `SIGTERM` for clean shutdown

**Example — filesystem MCP server:**

```bash
mcp-breaker wrap -- npx -y @modelcontextprotocol/server-filesystem /Users/you/projects
```

**Example — custom Node server:**

```bash
mcp-breaker wrap -- node /path/to/my_mcp_server.js
```

---

## Client Configuration

Replace the MCP server command in your client config with `mcp-breaker wrap -- …`.

### Cursor

Edit your MCP settings (Settings → MCP, or `~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/absolute/path/to/mcp-breaker",
      "args": [
        "wrap", "--",
        "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"
      ]
    }
  }
}
```

**Demo config (built-in test server, no npm):**

```json
{
  "mcpServers": {
    "demo": {
      "command": "/absolute/path/to/mcp-breaker",
      "args": [
        "wrap", "--",
        "/absolute/path/to/mcp-breaker/fakemcp"
      ]
    }
  }
}
```

Build `fakemcp` first: `go build -o fakemcp ./internal/testmcp/cmd/fakemcp`

Restart Cursor after saving the config.

### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/absolute/path/to/mcp-breaker",
      "args": [
        "wrap", "--",
        "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"
      ]
    }
  }
}
```

### Before / after

```json
// BEFORE — direct server
"postgres-server": {
  "command": "node",
  "args": ["postgres_mcp.js"]
}

// AFTER — wrapped with mcp-breaker
"postgres-server": {
  "command": "mcp-breaker",
  "args": ["wrap", "--", "node", "postgres_mcp.js"]
}
```

---

## How the Echo Breaker Works

The echo detector intercepts every incoming `tools/call` JSON-RPC request:

1. **Extract** the tool name (`params.name`) and arguments (`params.arguments`)
2. **Hash** the canonical argument payload (SHA-256 of `toolName:arguments`)
3. **Track** per-tool state: consecutive identical hash count + ring buffer of last 5 hashes
4. **Trip** on the 3rd consecutive identical call for the same tool
5. **Block** the request — it never reaches the child server
6. **Return** a synthetic MCP result with `isError: true`:

```json
{
  "jsonrpc": "2.0",
  "id": "<matching-request-id>",
  "result": {
    "content": [{
      "type": "text",
      "text": "Error: Command [write_file] generated identical failures across consecutive loops. Do not retry without modifying parameters."
    }],
    "isError": true
  }
}
```

**Reset conditions:**

- Different arguments for the same tool reset the consecutive counter
- Different tools maintain independent counters
- Non-`tools/call` methods (`initialize`, `tools/list`, notifications) always pass through unchanged

---

## Project Structure

```
mcp-breaker/
├── cmd/mcp-breaker/          # CLI: wrap, dashboard, models
├── pkg/
│   ├── proxy/                # JSON-RPC framing, session proxy, stdio wrap
│   ├── breaker/              # Echo, semantic, graph ledger, chain
│   ├── embed/                # ONNX/mock embedder, model download
│   ├── telemetry/            # Event bus, control socket
│   ├── dashboard/            # Bubble Tea TUI
│   ├── config/               # Runtime paths and thresholds
│   └── protocol/             # MCP types and intervention payloads
├── Formula/                  # Homebrew formula
├── internal/testmcp/         # Fake MCP server for tests and demos
├── scripts/demo.sh           # One-command local demo
├── docs/                     # Release notes, install guides
├── spec.md                   # Full product specification
├── Makefile
└── README.md
```

---

## Development

```bash
make build    # compile ./mcp-breaker
make test     # run all tests (framing, proxy, echo, integration)
make validate # full local validation (tests + echo + semantic + graph)
make negative-tests # simulate stuck-agent MCP traffic (echo, graph, semantic)
make validate-all # validate + negative-tests + UI e2e (headless)
make dev-ui         # Streamlit lab — wrap + dashboard in one browser window
make test-ui-e2e    # headless E2E for dev lab backend only
make vet      # go vet
make demo     # interactive local demo
make clean    # remove build artifacts
```

### Test coverage

| Suite | What it validates |
|-------|-------------------|
| `pkg/proxy` | NDJSON framing, split delivery, malformed frame skip, oversize rejection, session proxy |
| `pkg/breaker` | Echo trip on 3rd identical call, independent tools, ring buffer, non-tools/call passthrough |
| `internal/testmcp` | End-to-end `mcp-breaker wrap -- fakemcp` integration |

Run everything locally:

```bash
make validate
```

---

## Test evidence

Captured from local runs on macOS (Aug 2026). Full terminal logs live in [`docs/evidence/`](docs/evidence/).

### Full validation (`make validate`)

All automated and live flows pass in one command (test suite, vet, echo, semantic, graph ledger, control resume).

![make validate output](docs/evidence/validate.svg)

### Echo breaker — Test Case A (`make demo`)

Third identical `write_file` call blocked; first two forward as `ok`.

![echo demo output](docs/evidence/demo-echo.svg)

### Wrap startup — detector stack

Echo + graph ledger active; semantic enabled with `MCP_BREAKER_MOCK_EMBED=1`.

![wrap stderr logs](docs/evidence/echo-wrap.svg)

### Graph ledger — ABAB loop

Fourth `tools/call` in `write → read → write → read` pattern blocked with pause message.

![graph loop block](docs/evidence/graph-loop.svg)

### Semantic stagnation — Test Case B

Mock embedder detects paraphrased port-8080 errors (N vs N−2 cosine ≥ 0.88).

![semantic test output](docs/evidence/semantic-test.svg)

### TUI dashboard (`mcp-breaker dashboard`)

Live metrics, event stream, graph trip alert, and `[R] Resume` control during a wrap session.

![dashboard TUI](docs/evidence/dashboard-tui.svg)

Regenerate evidence after code changes:

```bash
make validate
python3 scripts/render_evidence.py
```

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, conventions, and the pull request process.

---

## Roadmap

- [x] Stdio JSON-RPC proxy with interceptor hooks
- [x] Tool parameter echo detector (spec Test Case A)
- [x] Semantic trajectory analysis with mock/ONNX embedder (spec Test Case B)
- [x] Sliding-window graph loop detection with dashboard resume
- [x] Terminal dashboard (`mcp-breaker dashboard`)
- [x] Homebrew formula and install docs
- [ ] `mcp-breaker init` — auto-rewrite client MCP configs
- [ ] Full in-process ONNX embedding pipeline (`-tags=onnx`)

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).

---

<p align="center">
  Built to stop agents from spinning in circles — so you can ship faster.
</p>
