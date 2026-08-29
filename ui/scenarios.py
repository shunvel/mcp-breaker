"""Canned MCP JSON-RPC scenarios for the dev lab."""

from __future__ import annotations

INIT = (
    '{"jsonrpc":"2.0","id":1,"method":"initialize",'
    '"params":{"protocolVersion":"2024-11-05","capabilities":{},'
    '"clientInfo":{"name":"dev-ui","version":"1.0.0"}}}'
)

SCENARIOS: dict[str, dict] = {
    "echo": {
        "title": "Echo loop",
        "badge": "Test Case A",
        "kind": "negative",
        "detector": "echo",
        "needs_semantic": False,
        "icon": "🔁",
        "summary": "Agent retries the exact same tool call after failures.",
        "description": (
            "Sends `write_file` three times with **identical arguments**. "
            "Calls 1–2 reach fakemcp (`ok`). Call 3 is **blocked** by the echo detector "
            "before it hits the server."
        ),
        "expects": ["Call 1 → forwarded", "Call 2 → forwarded", "Call 3 → echo block"],
        "frames": [
            INIT,
            '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/x","content":"same"}}}',
            '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/x","content":"same"}}}',
            '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"/tmp/x","content":"same"}}}',
        ],
    },
    "graph": {
        "title": "Graph loop (ABAB)",
        "badge": "Graph ledger",
        "kind": "negative",
        "detector": "graph",
        "needs_semantic": False,
        "icon": "🔀",
        "summary": "Agent ping-pongs between two tools without making progress.",
        "description": (
            "Alternates `write_file` → `read_file` → `write_file` → `read_file`. "
            "The graph ledger trips on the **4th call**, blocks it, and **pauses** the session. "
            "Use **Resume** in the dashboard to continue."
        ),
        "expects": ["Calls 1–3 → forwarded", "Call 4 → graph block + pause"],
        "frames": [
            INIT,
            '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"p":"a"}}}',
            '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"p":"b"}}}',
            '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"p":"a"}}}',
            '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_file","arguments":{"p":"b"}}}',
        ],
    },
    "semantic": {
        "title": "Semantic stagnation",
        "badge": "Test Case B",
        "kind": "negative",
        "detector": "semantic",
        "needs_semantic": True,
        "icon": "🧠",
        "summary": "Tool returns the same error meaning with different wording.",
        "description": (
            "Calls `simulate_port_error` three times with different args (so echo/graph stay quiet). "
            "fakemcp returns paraphrased port-8080 errors. On turn 3, semantic detector **appends** "
            "a reasoning alert to the response (requires mock embedder)."
        ),
        "expects": ["Turn 1–2 → error text only", "Turn 3 → error + semantic alert"],
        "frames": [
            INIT,
            '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"simulate_port_error","arguments":{"attempt":1}}}',
            '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"simulate_port_error","arguments":{"attempt":2}}}',
            '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"simulate_port_error","arguments":{"attempt":3}}}',
        ],
    },
    "healthy": {
        "title": "Healthy agent",
        "badge": "Control",
        "kind": "control",
        "detector": "none",
        "needs_semantic": False,
        "icon": "✅",
        "summary": "Normal traffic — nothing should trip.",
        "description": (
            "Three `write_file` calls with **different arguments** each time. "
            "All calls should forward cleanly with no blocks or alerts."
        ),
        "expects": ["All calls → forwarded (ok)"],
        "frames": [
            INIT,
            '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write_file","arguments":{"n":1}}}',
            '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_file","arguments":{"n":2}}}',
            '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"write_file","arguments":{"n":3}}}',
        ],
    },
}

SCENARIO_GROUPS = {
    "Stuck-agent scenarios": ["echo", "graph", "semantic"],
    "Control (should pass)": ["healthy"],
}

DETECTOR_INFO = {
    "echo": ("Echo detector", "Blocks identical tools/call payloads (≥3 in a row)", "#f59e0b"),
    "graph": ("Graph ledger", "Blocks A→B→A→B tool ping-pong loops", "#ef4444"),
    "semantic": ("Semantic tracker", "Flags paraphrased repeated failures (N vs N−2)", "#8b5cf6"),
    "none": ("No trip expected", "Traffic passes through unchanged", "#22c55e"),
}
