#!/usr/bin/env python3
"""Render terminal text captures as SVG images for README evidence."""
import html
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
EVIDENCE = ROOT / "docs" / "evidence"

ANSI = re.compile(r"\x1b\[[0-9;]*m")


def strip_ansi(text: str) -> str:
    return ANSI.sub("", text)


def color_for(line: str) -> str:
    if line.startswith("PASS"):
        return "#7dcea0"
    if line.startswith("FAIL") or "Error:" in line:
        return "#e74c3c"
    if line.startswith("===") or line.startswith("──"):
        return "#85c1e9"
    if "BLOCKED" in line or "graph_trip" in line or "PAUSED" in line:
        return "#f5b041"
    return "#d5dbdb"


def render(text: str, out: Path, title: str = "", max_lines: int = 32) -> None:
    lines = strip_ansi(text).splitlines()
    if title:
        lines = [title, "─" * min(72, max(len(title), 40))] + lines
    lines = lines[:max_lines]
    line_h = 16
    pad = 16
    width = min(960, max(520, max((len(l) for l in lines), default=40) * 7 + pad * 2))
    height = pad * 2 + len(lines) * line_h
    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}">',
        f'<rect width="100%" height="100%" fill="#1e1e1e" rx="8"/>',
    ]
    y = pad + 12
    for line in lines:
        fill = color_for(line)
        safe = html.escape(line[:120])
        parts.append(f'<text x="{pad}" y="{y}" fill="{fill}" font-family="Menlo,Monaco,monospace" font-size="12">{safe}</text>')
        y += line_h
    parts.append("</svg>")
    out.write_text("\n".join(parts), encoding="utf-8")
    print(f"wrote {out}")


def main() -> None:
    captures = [
        ("validate-terminal.txt", "validate.svg", "make validate — all flows passed"),
        ("demo-terminal.txt", "demo-echo.svg", "make demo — echo breaker (Test Case A)"),
        ("echo-wrap-stderr.txt", "echo-wrap.svg", "wrap stderr — detector startup logs"),
        ("graph-wrap-stdout.txt", "graph-loop.svg", "graph ledger — ABAB loop blocked"),
        ("semantic-test.txt", "semantic-test.svg", "Test Case B — semantic stagnation"),
        ("dashboard-terminal.txt", "dashboard-tui.svg", "mcp-breaker dashboard — live TUI"),
    ]
    for src_name, svg_name, title in captures:
        src = EVIDENCE / src_name
        if not src.exists():
            print(f"skip missing {src}", file=sys.stderr)
            continue
        text = src.read_text(errors="replace")
        if text.startswith("---"):
            parts = text.split("---", 2)
            if len(parts) >= 3:
                text = parts[2].lstrip("\n")
        render(text, EVIDENCE / svg_name, title)


if __name__ == "__main__":
    main()
