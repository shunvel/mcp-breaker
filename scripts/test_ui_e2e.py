#!/usr/bin/env python3
"""End-to-end tests for the Streamlit dev lab backend (no browser required)."""

from __future__ import annotations

import ast
import queue
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "ui"))

from scenarios import SCENARIOS  # noqa: E402
from telemetry_client import DashboardState, EventListener, default_run_dir, drain_event_queue  # noqa: E402
from wrap_session import WrapSession  # noqa: E402


def classify(obj: dict) -> str:
    result = obj.get("result") or {}
    if "serverInfo" in result:
        return "init"
    text = (result.get("content") or [{}])[0].get("text", "")
    if "identical failures" in text:
        return "echo_block"
    if "Detected tool invocation loop" in text:
        return "graph_block"
    if "CRITICAL REASONING ALERT" in text:
        return "semantic_alert"
    if text == "ok":
        return "ok"
    if any(k in text for k in ("8080", "EADDRINUSE", "bind")):
        return "port_error"
    return "other"


def run_scenario(scenario_id: str, mock_embed: bool = True) -> tuple[list[dict], DashboardState]:
    needs_semantic = bool(SCENARIOS[scenario_id].get("needs_semantic", False))
    event_q: queue.Queue = queue.Queue()
    dash = DashboardState()
    listener = EventListener(default_run_dir() / "events.sock", event_q)
    wrap = WrapSession(root=ROOT, mock_embed=mock_embed)

    listener.start()
    assert listener.active, f"listener failed: {listener.last_error}"
    wrap.start()
    assert wrap.running(), "wrap failed to start"
    wrap.restart(mock_embed=mock_embed, semantic=needs_semantic)
    time.sleep(0.35)

    if needs_semantic:
        assert wrap.semantic_enabled(), "semantic detector not enabled (need MCP_BREAKER_MOCK_EMBED=1)"

    raw = wrap.send_frames(list(SCENARIOS[scenario_id]["frames"]))

    for _ in range(10):
        drain_event_queue(event_q, dash)
        time.sleep(0.05)

    wrap.stop()
    listener.stop()
    return raw, dash


def assert_echo(raw: list[dict]) -> None:
    kinds = [classify(o) for o in raw if classify(o) != "init"]
    assert kinds.count("ok") == 2, f"echo: want 2 ok, got {kinds}"
    assert kinds.count("echo_block") == 1, f"echo: want 1 block, got {kinds}"


def assert_graph(raw: list[dict]) -> None:
    kinds = [classify(o) for o in raw if classify(o) != "init"]
    assert kinds.count("ok") == 3, f"graph: want 3 ok, got {kinds}"
    assert kinds.count("graph_block") == 1, f"graph: want 1 block, got {kinds}"


def assert_semantic(raw: list[dict]) -> None:
    kinds = [classify(o) for o in raw if classify(o) != "init"]
    assert kinds.count("port_error") == 2, f"semantic: want 2 port errors, got {kinds}"
    assert kinds.count("semantic_alert") == 1, f"semantic: want 1 alert, got {kinds}"


def assert_healthy(raw: list[dict]) -> None:
    kinds = [classify(o) for o in raw if classify(o) != "init"]
    assert all(k == "ok" for k in kinds), f"healthy: want all ok, got {kinds}"


def assert_telemetry(dash: DashboardState, min_frames: int = 1) -> None:
    assert dash.metrics.get("total_frames", 0) >= min_frames, f"telemetry frames: {dash.metrics}"
    assert len(dash.events) >= min_frames, f"telemetry events: {dash.events}"


def test_semantic_after_graph() -> None:
    event_q: queue.Queue = queue.Queue()
    listener = EventListener(default_run_dir() / "events.sock", event_q)
    wrap = WrapSession(root=ROOT, mock_embed=True)
    listener.start()
    wrap.start()
    wrap.send_frames(list(SCENARIOS["graph"]["frames"]))
    wrap.restart(mock_embed=True, semantic=True)
    time.sleep(0.35)
    assert wrap.semantic_enabled()
    raw = wrap.send_frames(list(SCENARIOS["semantic"]["frames"]))
    assert_semantic(raw)
    wrap.stop()
    listener.stop()


def test_semantic_without_mock() -> None:
    wrap = WrapSession(root=ROOT, mock_embed=False, semantic=True)
    wrap.start()
    time.sleep(0.25)
    assert not wrap.semantic_enabled()
    raw = wrap.send_frames(list(SCENARIOS["semantic"]["frames"]))
    kinds = [classify(o) for o in raw if classify(o) != "init"]
    assert "semantic_alert" not in kinds, kinds
    assert kinds.count("port_error") == 3, kinds
    wrap.stop()


def main() -> int:
    print("=== UI E2E: all scenarios ===")
    for sid, fn in [
        ("echo", assert_echo),
        ("graph", assert_graph),
        ("semantic", assert_semantic),
        ("healthy", assert_healthy),
    ]:
        raw, dash = run_scenario(sid)
        fn(raw)
        assert_telemetry(dash)
        print(f"  PASS  {SCENARIOS[sid]['title']}")

    print("\n=== UI E2E: cross-scenario ===")
    test_semantic_after_graph()
    print("  PASS  semantic after graph (proxy restart)")
    test_semantic_without_mock()
    print("  PASS  semantic without mock embed")

    print("\n=== UI E2E: static checks ===")
    ast.parse((ROOT / "ui" / "app.py").read_text())
    print("  PASS  ui/app.py syntax")
    for path in ["ui/wrap_session.py", "ui/telemetry_client.py", "ui/scenarios.py"]:
        ast.parse((ROOT / path).read_text())
        print(f"  PASS  {path} syntax")

    print("\nAll UI E2E checks passed.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except AssertionError as exc:
        print(f"\nFAIL  {exc}", file=sys.stderr)
        raise SystemExit(1)
