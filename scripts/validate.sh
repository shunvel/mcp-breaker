#!/usr/bin/env bash
# validate.sh — run all core validation flows (1–4) in one command.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO="${GO:-go}"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
BOLD='\033[1m'
NC='\033[0m'

step() { echo -e "\n${BOLD}=== $1 ===${NC}"; }
pass() { echo -e "${GREEN}PASS${NC}  $1"; }
fail() { echo -e "${RED}FAIL${NC}  $1" >&2; exit 1; }

step "Flow 1: Full test suite"
"$GO" test ./... -count=1
pass "go test ./..."

step "Flow 2: Static analysis"
"$GO" vet ./...
pass "go vet ./..."

step "Build binaries"
"$GO" build -o "$ROOT/mcp-breaker" ./cmd/mcp-breaker
"$GO" build -o "$ROOT/fakemcp" ./internal/testmcp/cmd/fakemcp
pass "mcp-breaker + fakemcp built"

step "Flow 3: Echo breaker live (Test Case A)"
VALIDATE_OUT="$ROOT/validate-stdout.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"validate","version":"1.0.0"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/demo.txt","content":"same payload"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/demo.txt","content":"same payload"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/demo.txt","content":"same payload"}}}'
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$VALIDATE_OUT"

python3 <<'PY'
import json, sys
from pathlib import Path
lines = Path("validate-stdout.ndjson").read_text().splitlines()
blocked = forwarded = initialized = 0
for line in lines:
    obj = json.loads(line)
    r = obj.get("result") or {}
    if "serverInfo" in r:
        initialized += 1
    elif "content" in r:
        text = r["content"][0].get("text", "")
        if "identical failures" in text:
            blocked += 1
        elif text == "ok":
            forwarded += 1
if initialized < 1:
    sys.exit("expected initialize response")
if forwarded != 2:
    sys.exit(f"expected 2 forwarded ok responses, got {forwarded}")
if blocked != 1:
    sys.exit(f"expected 1 echo block, got {blocked}")
print("  initialize=1 forwarded=2 blocked=1")
PY
pass "echo trip on 3rd identical tools/call"

step "Flow 4a: Semantic stagnation (Test Case B)"
MCP_BREAKER_MOCK_EMBED=1 "$GO" test ./pkg/breaker -run TestCaseB_SemanticStagnation -count=1
pass "semantic Test Case B"

step "Flow 4b: Mock embedder cosine"
"$GO" test ./pkg/embed -run TestMockEmbedderPort8080 -count=1
pass "mock embedder similarity >= 0.88"

step "Flow 5: Graph ledger ABAB + resume"
"$GO" test ./pkg/breaker -run 'TestGraphLedger|TestDetectGraphLoop' -count=1
pass "graph ledger unit tests"

GRAPH_OUT="$ROOT/validate-graph.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"validate","version":"1.0.0"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"p":"a"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"p":"b"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"p":"a"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_file","arguments":{"p":"b"}}}'
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$GRAPH_OUT"

python3 <<'PY'
import json, sys
from pathlib import Path
blocked = forwarded = 0
for line in Path("validate-graph.ndjson").read_text().splitlines():
    obj = json.loads(line)
    r = obj.get("result") or {}
    if "content" not in r:
        continue
    text = r["content"][0].get("text", "")
    if "Detected tool invocation loop" in text:
        blocked += 1
    elif text == "ok":
        forwarded += 1
if forwarded != 3:
    sys.exit(f"expected 3 forwarded ok before graph trip, got {forwarded}")
if blocked != 1:
    sys.exit(f"expected 1 graph block, got {blocked}")
print("  forwarded=3 graph_blocked=1")
PY
pass "graph loop blocked on 4th tools/call (ABAB)"

step "Flow 6: Control resume socket"
"$GO" test ./pkg/telemetry -run TestControlResume -count=1
pass "dashboard resume control socket"

echo ""
echo -e "${GREEN}${BOLD}All validations passed.${NC}"
echo "Optional manual checks:"
echo "  ./mcp-breaker dashboard          # TUI (second terminal while wrap runs)"
echo "  ./mcp-breaker models download    # ONNX model cache"
echo "  MCP_BREAKER_MOCK_EMBED=1 ./mcp-breaker wrap -- ./fakemcp"
