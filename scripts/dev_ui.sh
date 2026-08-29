#!/usr/bin/env bash
# Start the Streamlit dev lab (wrap + dashboard in one browser window).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "Installing UI dependencies (streamlit)..."
python3 -m pip install -q -r ui/requirements.txt

if [[ ! -x "$ROOT/mcp-breaker" || ! -x "$ROOT/fakemcp" ]]; then
  echo "Building mcp-breaker + fakemcp..."
  make build
  go build -o "$ROOT/fakemcp" ./internal/testmcp/cmd/fakemcp
fi

echo ""
echo "Starting dev lab at http://localhost:8501"
echo "Press Ctrl+C to stop."
echo ""

cd "$ROOT/ui"
exec python3 -m streamlit run app.py \
  --server.address localhost \
  --server.port 8501 \
  --browser.gatherUsageStats false
