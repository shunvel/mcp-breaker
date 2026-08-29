#!/usr/bin/env bash
# negative_tests.sh — simulate "stuck agent" MCP traffic and verify detections.
#
# Each scenario pipes JSON-RPC NDJSON into `mcp-breaker wrap -- fakemcp` (or runs
# a focused unit test for semantic stagnation). Use this to see *what* the proxy
# inspects and *when* it intervenes.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO="${GO:-go}"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

section() { echo -e "\n${BOLD}━━━ $1 ━━━${NC}"; }
info()    { echo -e "${YELLOW}→${NC} $*"; }
pass()    { echo -e "${GREEN}DETECTED${NC}  $1"; }
ok()      { echo -e "${GREEN}CLEAN${NC}     $1"; }
fail()    { echo -e "${RED}UNEXPECTED${NC} $1" >&2; exit 1; }

build() {
  "$GO" build -o "$ROOT/mcp-breaker" ./cmd/mcp-breaker
  "$GO" build -o "$ROOT/fakemcp" ./internal/testmcp/cmd/fakemcp
}

parse_responses() {
  python3 - "$1" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1])
for line in path.read_text().splitlines():
    obj = json.loads(line)
    rid = obj.get("id")
    r = obj.get("result") or {}
    if "serverInfo" in r:
        print(f"  id={rid} initialize")
        continue
    if "content" not in r:
        continue
    text = r["content"][0].get("text", "")
    if "identical failures" in text:
        print(f"  id={rid} [ECHO BLOCK] {text[:72]}...")
    elif "Detected tool invocation loop" in text:
        print(f"  id={rid} [GRAPH BLOCK] {text[:72]}...")
    elif "CRITICAL REASONING ALERT" in text:
        print(f"  id={rid} [SEMANTIC ALERT] {text[:72]}...")
    elif text == "ok":
        print(f"  id={rid} forwarded → ok")
    else:
        print(f"  id={rid} {text[:72]}")
PY
}

section "Build"
build
info "Proxy sits between your MCP client (stdin) and the real server (fakemcp child process)."
info "Client → server frames hit echo + graph ledger. Server → client frames hit semantic."

# ── Negative case 1: Echo loop (Test Case A) ─────────────────────────────────
section "Negative 1 — Echo loop (identical tools/call payload × 3)"
info "Bad agent behavior: retry write_file with the exact same arguments after failure."
info "Internal: SHA256(toolName + arguments); trip when consecutiveCount ≥ 3 per tool."

OUT="$ROOT/negative-echo.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"neg","version":"1"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/x","content":"same"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/x","content":"same"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/x","content":"same"}}}'
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$OUT"

parse_responses "$OUT"
python3 - "$OUT" <<'PY' || fail "echo loop should block on 3rd identical call"
import json, sys
from pathlib import Path
blocked = sum(1 for line in Path(sys.argv[1]).read_text().splitlines()
              if "identical failures" in json.loads(line).get("result",{}).get("content",[{}])[0].get("text",""))
if blocked != 1: sys.exit(f"want 1 echo block, got {blocked}")
PY
pass "Echo detector blocked the 3rd identical write_file (never reached fakemcp)."

# ── Negative case 2: Graph ABAB loop ─────────────────────────────────────────
section "Negative 2 — Graph loop (write → read → write → read)"
info "Bad agent behavior: alternate two tools with fixed args — classic A-B-A-B ping-pong."
info "Internal: ledger stores tool:hash keys; tail A,B,A,B with A≠B trips and pauses session."

OUT="$ROOT/negative-graph.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"neg","version":"1"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"p":"a"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"p":"b"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"p":"a"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_file","arguments":{"p":"b"}}}'
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$OUT"

parse_responses "$OUT"
python3 - "$OUT" <<'PY' || fail "graph loop should block on 4th call"
import json, sys
from pathlib import Path
lines = [json.loads(l) for l in Path(sys.argv[1]).read_text().splitlines()]
fwd = sum(1 for o in lines if o.get("result",{}).get("content",[{}])[0].get("text")=="ok")
blk = sum(1 for o in lines if "Detected tool invocation loop" in o.get("result",{}).get("content",[{}])[0].get("text",""))
if fwd != 3: sys.exit(f"want 3 forwarded, got {fwd}")
if blk != 1: sys.exit(f"want 1 graph block, got {blk}")
PY
pass "Graph ledger blocked the 4th call and paused (resume via dashboard [R])."

# ── Negative case 3: Semantic stagnation live via wrap (Test Case B) ─────────
section "Negative 3 — Semantic stagnation live (simulate_port_error × 3)"
info "Bad agent behavior: tool keeps returning paraphrased port-8080 bind failures."
info "fakemcp tool simulate_port_error rotates error texts; args differ so echo/graph stay quiet."
info "Internal: MCP_BREAKER_MOCK_EMBED=1 enables mock embedder on server→client path."

OUT="$ROOT/negative-semantic.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"neg","version":"1"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"simulate_port_error","arguments":{"attempt":1}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"simulate_port_error","arguments":{"attempt":2}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"simulate_port_error","arguments":{"attempt":3}}}'
} | MCP_BREAKER_MOCK_EMBED=1 "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$OUT"

parse_responses "$OUT"
python3 - "$OUT" <<'PY' || fail "semantic live wrap should alert on 3rd response"
import json, sys
from pathlib import Path
alerts = clean = 0
for line in Path(sys.argv[1]).read_text().splitlines():
    text = json.loads(line).get("result",{}).get("content",[{}])[0].get("text","")
    if "CRITICAL REASONING ALERT" in text:
        alerts += 1
    elif "8080" in text or "EADDRINUSE" in text or "bind" in text:
        clean += 1
if alerts != 1:
    sys.exit(f"want 1 semantic alert on turn 3, got {alerts}")
if clean < 2:
    sys.exit(f"want at least 2 plain port errors before alert, got {clean}")
PY
pass "Semantic detector appended CRITICAL REASONING ALERT on 3rd live wrap response."

section "Negative 3b — Semantic stagnation (unit test cross-check)"
MCP_BREAKER_MOCK_EMBED=1 "$GO" test ./pkg/breaker -run TestCaseB_SemanticStagnation -count=1
pass "Test Case B unit test still passes."

# ── Control 1: varying args — echo should NOT trip ───────────────────────────
section "Control 1 — Varying arguments (healthy agent)"
info "Good agent behavior: change parameters each attempt. Echo consecutive counter resets."

OUT="$ROOT/negative-control-echo.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"neg","version":"1"}}}'
  for i in 1 2 3 4 5; do
    printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":$((i+1)),\"method\":\"tools/call\",\"params\":{\"name\":\"write_file\",\"arguments\":{\"n\":$i}}}"
  done
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$OUT"

parse_responses "$OUT"
python3 - "$OUT" <<'PY' || fail "varying args should never trip echo"
import json, sys
from pathlib import Path
for line in Path(sys.argv[1]).read_text().splitlines():
    text = json.loads(line).get("result",{}).get("content",[{}])[0].get("text","")
    if "identical failures" in text or "Detected tool invocation loop" in text:
        sys.exit("unexpected block")
PY
ok "Five distinct payloads — all forwarded, no false positive."

# ── Control 2: non-ABAB sequence ─────────────────────────────────────────────
section "Control 2 — Non-alternating tool sequence"
info "Good agent behavior: progress through different tools — no A-B-A-B tail pattern."

OUT="$ROOT/negative-control-graph.ndjson"
{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"neg","version":"1"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"step":1}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"step":2}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_dir","arguments":{"step":3}}}'
  printf '%s\n' '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"grep","arguments":{"step":4}}}'
} | "$ROOT/mcp-breaker" wrap -- "$ROOT/fakemcp" 2>/dev/null > "$OUT"

parse_responses "$OUT"
python3 - "$OUT" <<'PY' || fail "linear tool progression should not trip graph ledger"
import json, sys
from pathlib import Path
for line in Path(sys.argv[1]).read_text().splitlines():
    text = json.loads(line).get("result",{}).get("content",[{}])[0].get("text","")
    if "Detected tool invocation loop" in text:
        sys.exit("unexpected graph block")
PY
ok "Linear write → read → list → grep — all forwarded."

echo ""
echo -e "${GREEN}${BOLD}All negative + control scenarios passed.${NC}"
echo "Tip: run ./mcp-breaker dashboard in another terminal while replaying Negative 2 to see pause/resume."
