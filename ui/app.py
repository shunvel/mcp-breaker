"""Streamlit dev lab — interactive wrap + dashboard tester."""

from __future__ import annotations

import html
import queue
import sys
import time
from pathlib import Path
from typing import Any

import streamlit as st

UI_DIR = Path(__file__).resolve().parent
ROOT = UI_DIR.parent
sys.path.insert(0, str(UI_DIR))

from scenarios import DETECTOR_INFO, SCENARIO_GROUPS, SCENARIOS  # noqa: E402
from telemetry_client import (  # noqa: E402
    DashboardState,
    EventListener,
    default_run_dir,
    drain_event_queue,
    send_resume,
)
from wrap_session import WrapSession  # noqa: E402

st.set_page_config(
    page_title="mcp-breaker lab",
    page_icon="⚡",
    layout="wide",
    initial_sidebar_state="collapsed",
)

CSS = """
<style>
    /* Prevent title clipping under Streamlit header */
    .block-container { padding-top: 3.5rem !important; padding-bottom: 2rem; max-width: 900px; }
    header[data-testid="stHeader"] { background: transparent; }
    .hero-sub { color: #475569; font-size: 1rem; margin-bottom: 1.25rem; margin-top: 0; }
    .quickstart {
        background: #eff6ff; border: 1px solid #93c5fd; border-radius: 8px;
        padding: 0.85rem 1rem; color: #1e3a8a; margin-bottom: 1.5rem; font-size: 0.95rem;
    }
    .progress-row {
        display: flex; gap: 0.35rem; align-items: center; flex-wrap: wrap;
        margin-bottom: 1.5rem; font-size: 0.85rem;
    }
    .prog-step {
        padding: 0.35rem 0.75rem; border-radius: 999px; border: 1px solid #cbd5e1;
        background: #f8fafc; color: #64748b;
    }
    .prog-step.done { background: #dcfce7; border-color: #86efac; color: #166534; font-weight: 600; }
    .prog-step.active { background: #dbeafe; border-color: #60a5fa; color: #1d4ed8; font-weight: 600; }
    .prog-arrow { color: #94a3b8; }
    .phase-title { font-size: 1.05rem; font-weight: 700; margin-bottom: 0.35rem; color: #0f172a; }
    .phase-desc { color: #64748b; font-size: 0.88rem; margin-bottom: 0.75rem; }
    .timeline-card {
        background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px;
        padding: 0.65rem 0.85rem; margin-bottom: 0.4rem; color: #1e293b;
    }
    .badge {
        display: inline-block; border-radius: 4px; padding: 0.12rem 0.5rem;
        font-size: 0.72rem; font-weight: 700; margin-right: 0.5rem;
    }
    .badge-ok       { background: #dcfce7; color: #166534; border: 1px solid #86efac; }
    .badge-echo     { background: #fef3c7; color: #92400e; border: 1px solid #fcd34d; }
    .badge-graph    { background: #fee2e2; color: #991b1b; border: 1px solid #fca5a5; }
    .badge-semantic { background: #ede9fe; color: #5b21b6; border: 1px solid #c4b5fd; }
    .badge-port    { background: #fce7f3; color: #9d174d; border: 1px solid #f9a8d4; }
</style>
"""


def ensure_session() -> None:
    if "wrap" not in st.session_state:
        st.session_state.wrap = WrapSession(root=ROOT, mock_embed=True)
    if "dash" not in st.session_state:
        st.session_state.dash = DashboardState()
    if "event_queue" not in st.session_state:
        st.session_state.event_queue = queue.Queue()
    if "listener" not in st.session_state:
        st.session_state.listener = EventListener(
            default_run_dir() / "events.sock", st.session_state.event_queue
        )
    if "raw_json" not in st.session_state:
        st.session_state.raw_json = []
    if "last_scenario" not in st.session_state:
        st.session_state.last_scenario = None


def sync_telemetry(wait_ms: int = 0) -> int:
    if wait_ms > 0:
        time.sleep(wait_ms / 1000)
    return drain_event_queue(st.session_state.event_queue, st.session_state.dash)


def start_lab(mock_embed: bool) -> None:
    wrap: WrapSession = st.session_state.wrap
    listener: EventListener = st.session_state.listener
    dash: DashboardState = st.session_state.dash

    st.session_state.event_queue = queue.Queue()
    listener.event_queue = st.session_state.event_queue
    wrap.mock_embed = mock_embed
    wrap.semantic = False
    listener.start()
    if listener.last_error:
        st.error(f"Could not start event listener: {listener.last_error}")
        return

    try:
        wrap.start()
    except FileNotFoundError as exc:
        st.error(str(exc))
        listener.stop()
        return

    dash.events.clear()
    dash.metrics = {}
    dash.paused = False
    dash.pattern = ""
    dash.last_sim = 0.0
    st.session_state.raw_json = []
    st.session_state.last_scenario = None
    st.toast("Proxy is running", icon="✅")


def stop_lab() -> None:
    st.session_state.wrap.stop()
    st.session_state.listener.stop()
    st.toast("Proxy stopped", icon="⏹️")


def classify_response(obj: dict[str, Any]) -> tuple[str, str, str, str]:
    rid = str(obj.get("id", "?"))
    result = obj.get("result") or {}
    if "serverInfo" in result:
        name = result.get("serverInfo", {}).get("name", "server")
        return rid, "badge-ok", "INIT", f"Connected to {name}"

    content = (result.get("content") or [{}])[0]
    text = content.get("text", "")
    if "identical failures" in text:
        return rid, "badge-echo", "ECHO BLOCK", text
    if "Detected tool invocation loop" in text:
        return rid, "badge-graph", "GRAPH BLOCK", text
    if "CRITICAL REASONING ALERT" in text:
        return rid, "badge-semantic", "SEMANTIC ALERT", text
    if any(k in text for k in ("8080", "EADDRINUSE", "bind")):
        return rid, "badge-port", "PORT ERROR", text
    if text == "ok":
        return rid, "badge-ok", "OK", "Forwarded — server returned ok"
    if not text:
        return rid, "badge-ok", "EMPTY", "(no text in response)"
    return rid, "badge-ok", "RESPONSE", text[:300]


def prepare_for_test(scenario_id: str, mock_embed: bool) -> str | None:
    """Reset proxy state before a test. Returns an error message or None."""
    wrap: WrapSession = st.session_state.wrap
    if not wrap.running():
        return "Start the proxy first (section 1)."

    if scenario_id == "semantic" and not mock_embed:
        return (
            "Semantic test needs **Mock semantic embedder** enabled in Settings. "
            "Turn it on, then click **Stop** and **Start proxy** again."
        )

    needs_semantic = bool(SCENARIOS[scenario_id].get("needs_semantic", False))

    # Fresh wrap process — clears graph pause and per-detector history from prior tests.
    wrap.restart(mock_embed=mock_embed, semantic=needs_semantic)
    time.sleep(0.35)

    if needs_semantic and not wrap.semantic_enabled():
        return (
            "Semantic detector did not start. Enable **Mock semantic embedder**, "
            "then **Stop** and **Start proxy** again."
        )
    return None


def render_timeline() -> None:
    if not st.session_state.raw_json:
        st.caption("Nothing yet — press **Run test** above after starting the proxy.")
        return
    for obj in st.session_state.raw_json:
        rid, badge_cls, badge_lbl, msg = classify_response(obj)
        safe_msg = html.escape(msg)
        st.markdown(
            f'<div class="timeline-card">'
            f'<span style="color:#64748b;font-family:monospace;margin-right:0.5rem;">#{html.escape(rid)}</span>'
            f'<span class="badge {badge_cls}">{html.escape(badge_lbl)}</span>'
            f'<span style="font-size:0.9rem;white-space:pre-wrap;">{safe_msg}</span>'
            f"</div>",
            unsafe_allow_html=True,
        )


def render_progress(running: bool, has_results: bool) -> None:
    s1 = "done" if running else "active"
    s2 = "active" if running else ""
    s3 = "done" if has_results else ("active" if running else "")
    s4 = "done" if has_results else ""
    st.markdown(
        f'<div class="progress-row">'
        f'<span class="prog-step {s1}">1 · Start proxy</span>'
        f'<span class="prog-arrow">→</span>'
        f'<span class="prog-step {s2}">2 · Pick test</span>'
        f'<span class="prog-arrow">→</span>'
        f'<span class="prog-step {s3}">3 · Run test</span>'
        f'<span class="prog-arrow">→</span>'
        f'<span class="prog-step {s4}">4 · See results</span>'
        f"</div>",
        unsafe_allow_html=True,
    )


@st.fragment(run_every=2)
def dashboard_panel(running: bool) -> None:
    listener: EventListener = st.session_state.listener
    dash: DashboardState = st.session_state.dash
    if running and not listener.active:
        listener.start()
    sync_telemetry()
    metrics = dash.metrics or {}

    if dash.paused:
        st.error(f"Graph loop paused the session: **{dash.pattern}**")
        if st.button("Resume session", type="primary", key="btn_resume", disabled=not running):
            try:
                send_resume(default_run_dir() / "control.sock")
                st.toast("Resumed", icon="▶️")
            except OSError as exc:
                st.error(str(exc))

    c1, c2, c3, c4 = st.columns(4)
    c1.metric("Frames seen", metrics.get("total_frames", 0))
    c2.metric("Tool calls", metrics.get("tools_call_count", 0))
    c3.metric("Blocks triggered", (
        metrics.get("echo_trips", 0) + metrics.get("semantic_trips", 0) + metrics.get("graph_trips", 0)
    ))
    c4.metric("Tokens saved", f"~{metrics.get('tokens_saved', 0)}")

    e1, e2, e3 = st.columns(3)
    e1.metric("Echo blocks", metrics.get("echo_trips", 0))
    e2.metric("Semantic alerts", metrics.get("semantic_trips", 0))
    e3.metric("Graph blocks", metrics.get("graph_trips", 0))

    if dash.events:
        st.code("\n".join(dash.events[-12:]), language="text")
    elif running:
        st.caption("Run a test — detector events will appear here.")
    else:
        st.caption("Start the proxy first.")


ensure_session()
wrap: WrapSession = st.session_state.wrap
listener: EventListener = st.session_state.listener

st.markdown(CSS, unsafe_allow_html=True)

with st.sidebar:
    st.header("Settings")
    mock_embed = st.toggle("Mock semantic embedder", value=True, key="mock_embed",
                           help="Required for the Semantic stagnation test")
    with st.expander("What is this page?"):
        st.markdown(
            "Simulates a **stuck AI agent** sending bad MCP tool calls.\n\n"
            "**mcp-breaker wrap** catches:\n"
            "- **Echo** — same call repeated\n"
            "- **Graph** — ping-pong between two tools\n"
            "- **Semantic** — same error, different words"
        )

st.title("mcp-breaker test lab")
st.markdown(
    '<div class="hero-sub">Simulate a stuck AI agent and watch mcp-breaker catch bad MCP tool calls.</div>',
    unsafe_allow_html=True,
)
st.markdown(
    '<div class="quickstart"><strong>Quick start:</strong> '
    "Start proxy → pick <em>Echo loop</em> → Run test → scroll down for results.</div>",
    unsafe_allow_html=True,
)

running = wrap.running()
has_results = bool(st.session_state.raw_json)
render_progress(running, has_results)

with st.container(border=True):
    st.markdown('<div class="phase-title">1 · Start the proxy</div>', unsafe_allow_html=True)
    st.markdown(
        '<div class="phase-desc">Launches <code>mcp-breaker wrap -- fakemcp</code>. '
        "The circuit breaker that inspects every tool call.</div>",
        unsafe_allow_html=True,
    )
    if running:
        st.success("Proxy is **running**.")
        st.caption("Telemetry: active" if listener.active else "Telemetry: not ready")
    else:
        st.warning("Proxy is **stopped** — click Start before running a test.")
    c1, c2, c3 = st.columns([1, 1, 2])
    with c1:
        if st.button("▶ Start proxy", type="primary", use_container_width=True, key="btn_start"):
            start_lab(mock_embed)
            st.rerun()
    with c2:
        if st.button("⏹ Stop", use_container_width=True, key="btn_stop"):
            stop_lab()
            st.rerun()
    if not (ROOT / "mcp-breaker").exists():
        c3.warning("Run `make build` first.")

with st.container(border=True):
    st.markdown('<div class="phase-title">2 · Pick a test scenario</div>', unsafe_allow_html=True)
    st.markdown('<div class="phase-desc">Each scenario mimics a different stuck-agent behavior.</div>', unsafe_allow_html=True)

    group = st.radio(
        "Type",
        list(SCENARIO_GROUPS.keys()),
        horizontal=True,
        key="scenario_group",
        format_func=lambda g: "Should catch a problem" if "Stuck" in g else "Should pass cleanly",
    )
    scenario_ids = SCENARIO_GROUPS[group]
    labels = {sid: f"{SCENARIOS[sid]['icon']} {SCENARIOS[sid]['title']}" for sid in scenario_ids}
    scenario_id = st.selectbox("Scenario", scenario_ids, format_func=lambda x: labels[x], key="scenario_id")
    scenario = SCENARIOS[scenario_id]

    det_name, det_desc, _ = DETECTOR_INFO.get(scenario["detector"], ("", "", "#64748b"))
    st.info(f"**{scenario['summary']}**  \n{scenario['description']}")
    st.caption(f"Detector: **{det_name}** — {det_desc}")
    st.markdown("**What should happen:**")
    for step in scenario["expects"]:
        st.markdown(f"- {step}")

    st.divider()
    st.markdown('<div class="phase-title">3 · Run the test</div>', unsafe_allow_html=True)
    st.markdown('<div class="phase-desc">Sends JSON-RPC tool calls through the running proxy.</div>', unsafe_allow_html=True)

    if scenario_id == "semantic" and not mock_embed:
        st.warning("Enable **Mock semantic embedder** in Settings for this test.")

    if st.button(
        f"▶ Run test: {scenario['title']}",
        type="primary",
        disabled=not running,
        use_container_width=True,
        key="btn_run",
    ):
        err = prepare_for_test(scenario_id, mock_embed)
        if err:
            st.error(err)
        else:
            try:
                raw = wrap.send_frames(list(scenario["frames"]))
                st.session_state.raw_json = raw
                st.session_state.last_scenario = scenario_id
                for _ in range(8):
                    if sync_telemetry(wait_ms=100) > 0:
                        break
                st.toast("Test complete — scroll down for results", icon="✅")
                st.rerun()
            except Exception as exc:  # noqa: BLE001
                st.error(f"Test failed: {exc}")

with st.container(border=True):
    st.markdown('<div class="phase-title">4 · Results</div>', unsafe_allow_html=True)
    st.markdown('<div class="phase-desc">What the proxy returned for each tool call.</div>', unsafe_allow_html=True)
    render_timeline()
    with st.expander("Raw JSON"):
        st.json(st.session_state.raw_json)
    with st.expander("Proxy logs"):
        st.code("\n".join(wrap.stderr_lines[-25:]) or "(empty)", language="text")

with st.container(border=True):
    st.markdown('<div class="phase-title">Live dashboard</div>', unsafe_allow_html=True)
    st.markdown('<div class="phase-desc">Detector events and counters (same as terminal dashboard).</div>', unsafe_allow_html=True)
    sync_telemetry()
    dashboard_panel(running)
