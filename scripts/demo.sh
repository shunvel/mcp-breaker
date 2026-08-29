#!/usr/bin/env bash
# Local demo — no MCP config or running servers required.
# Uses the built-in fakemcp binary and pipes JSON-RPC through mcp-breaker.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export PATH="${HOME}/sdk/go/bin:${PATH}"

cd "$ROOT"

echo "Building mcp-breaker and fakemcp..."
go build -o "$ROOT/mcp-breaker" ./cmd/mcp-breaker
go build -o "$ROOT/fakemcp" ./internal/testmcp/cmd/fakemcp

echo ""
echo "=== Demo: 3 identical write_file calls ==="
echo "Expected: calls 1-2 forward (ok), call 3 blocked (intervention)"
echo ""

{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"1.0.0"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/demo.txt","content":"same payload"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/demo.txt","content":"same payload"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/demo.txt","content":"same payload"}}}'
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>"$ROOT/demo-stderr.log" | tee "$ROOT/demo-stdout.ndjson"

echo ""
echo "--- Parsed responses ---"
python3 <<'PY'
import json
from pathlib import Path
for line in Path("demo-stdout.ndjson").read_text().splitlines():
    obj = json.loads(line)
    rid = obj.get("id")
    r = obj.get("result") or {}
    if "serverInfo" in r:
        print(f"  id={rid}: initialize -> {r['serverInfo']['name']}")
    elif "content" in r:
        text = r["content"][0]["text"]
        blocked = "identical failures" in text
        label = "BLOCKED" if blocked else "forwarded"
        print(f"  id={rid}: [{label}] {text[:70]}{'...' if len(text) > 70 else ''}")
PY

echo ""
echo "Done. No external MCP server was needed — fakemcp ships with this repo."
