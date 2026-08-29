# mcp-breaker

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
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
| **Graceful intervention** | Returns MCP `isError` results instead of crashing the session |
| **Built-in test server** | `fakemcp` for local demos with zero external dependencies |

### Planned

| Module | Description |
|--------|-------------|
| **Semantic stagnation tracker** | Local ONNX embeddings (All-MiniLM-L6-v2), cosine similarity ≥ 0.88 |
| **Sliding-window ledger** | Graph loop detection across turn sequences (A → B → A → B) |
| **Config auto-wrap (`init`)** | Rewrites Cursor / Claude Desktop MCP configs automatically |
| **TUI dashboard** | Real-time stream metrics and loop statistics |
| **Homebrew formula** | One-command install (`brew install mcp-breaker`) |

See [spec.md](spec.md) for the full product specification.

---

## Requirements

- **Go 1.23+** — [go.dev/dl](https://go.dev/dl/)
- **macOS / Linux** — stdio proxy (Windows support planned)
- **Optional:** Node.js + `npx` — only if wrapping npm-based MCP servers

---

## Installation

### From source

```bash
git clone https://github.com/shunvel/mcp-breaker.git
cd mcp-breaker
make build
```

This produces a single static binary at `./mcp-breaker` (~3 MB, zero runtime dependencies).

### Verify the build

```bash
make test    # run the full test suite
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

---

## CLI Reference

```
mcp-breaker — semantic MCP circuit breaker proxy

Usage:
  mcp-breaker wrap -- <command> [args...]
  mcp-breaker help

Commands:
  wrap    Launch <command> as a child MCP server and proxy stdio through
          the circuit breaker layer.
  help    Show usage information.
```

### `wrap`

```bash
mcp-breaker wrap -- <command> [args...]
```

| Flag / argument | Description |
|-----------------|-------------|
| `--` | Required separator. Everything after `--` is passed to the child process verbatim. |
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
├── cmd/mcp-breaker/          # CLI entry point
├── pkg/
│   ├── proxy/                # JSON-RPC framing, session proxy, stdio wrap
│   ├── breaker/              # Echo detector, semantic tracker (planned)
│   └── protocol/             # MCP types and intervention payloads
├── internal/testmcp/         # Fake MCP server for tests and demos
├── scripts/demo.sh           # One-command local demo
├── spec.md                   # Full product specification
├── Makefile
└── README.md
```

---

## Development

```bash
make build    # compile ./mcp-breaker
make test     # run all tests (framing, proxy, echo, integration)
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

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, conventions, and the pull request process.

---

## Roadmap

- [x] Stdio JSON-RPC proxy with interceptor hooks
- [x] Tool parameter echo detector (spec Test Case A)
- [x] Built-in fake MCP server and demo script
- [ ] Semantic trajectory analysis via local ONNX embeddings (spec Test Case B)
- [ ] Sliding-window graph loop detection
- [ ] `mcp-breaker init` — auto-rewrite client MCP configs
- [ ] Terminal dashboard (`mcp-breaker dashboard`)
- [ ] Homebrew formula and shell installer

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).

---

<p align="center">
  Built to stop agents from spinning in circles — so you can ship faster.
</p>
