"""Manage a long-lived mcp-breaker wrap subprocess."""

from __future__ import annotations

import json
import os
import subprocess
import threading
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class WrapSession:
    root: Path
    mock_embed: bool = True
    semantic: bool = False
    proc: subprocess.Popen[str] | None = None
    stderr_lines: list[str] = field(default_factory=list)
    _stderr_thread: threading.Thread | None = None

    @property
    def mcp_breaker(self) -> Path:
        return self.root / "mcp-breaker"

    @property
    def fakemcp(self) -> Path:
        return self.root / "fakemcp"

    def running(self) -> bool:
        return self.proc is not None and self.proc.poll() is None

    def start(self) -> None:
        self.stop()
        if not self.mcp_breaker.exists() or not self.fakemcp.exists():
            raise FileNotFoundError(
                "Build binaries first: make build (needs ./mcp-breaker and ./fakemcp)"
            )

        env = os.environ.copy()
        if self.mock_embed:
            env["MCP_BREAKER_MOCK_EMBED"] = "1"
        else:
            env.pop("MCP_BREAKER_MOCK_EMBED", None)

        cmd = [str(self.mcp_breaker.resolve()), "wrap"]
        if not self.semantic:
            cmd.append("--no-semantic")
        cmd.extend(["--", str(self.fakemcp.resolve())])

        self.proc = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
            env=env,
            cwd=str(self.root),
        )
        self.stderr_lines = []
        self._stderr_thread = threading.Thread(target=self._drain_stderr, daemon=True)
        self._stderr_thread.start()

    def restart(self, mock_embed: bool | None = None, semantic: bool | None = None) -> None:
        """Restart wrap subprocess with a clean detector state."""
        if mock_embed is not None:
            self.mock_embed = mock_embed
        if semantic is not None:
            self.semantic = semantic
        self.stop()
        self.start()

    def semantic_enabled(self) -> bool:
        return any("semantic detector: enabled" in line for line in self.stderr_lines)

    def stop(self) -> None:
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=2)
            except subprocess.TimeoutExpired:
                self.proc.kill()
        self.proc = None

    def _drain_stderr(self) -> None:
        if not self.proc or not self.proc.stderr:
            return
        for line in self.proc.stderr:
            self.stderr_lines.append(line.rstrip())
            if len(self.stderr_lines) > 100:
                self.stderr_lines = self.stderr_lines[-100:]

    def send_frames(self, frames: list[str]) -> list[dict[str, Any]]:
        if not self.running() or not self.proc or not self.proc.stdin or not self.proc.stdout:
            raise RuntimeError("wrap session is not running")

        responses: list[dict[str, Any]] = []
        for frame in frames:
            self.proc.stdin.write(frame.strip() + "\n")
            self.proc.stdin.flush()
            line = self.proc.stdout.readline()
            if not line:
                raise RuntimeError("wrap closed stdout unexpectedly")
            responses.append(json.loads(line))
        return responses

    @staticmethod
    def summarize_response(obj: dict[str, Any]) -> str:
        rid = obj.get("id")
        result = obj.get("result") or {}
        if "serverInfo" in result:
            name = result["serverInfo"].get("name", "?")
            return f"id={rid} initialize → {name}"
        content = result.get("content") or []
        if content:
            text = content[0].get("text", "")
            if "identical failures" in text:
                return f"id={rid} [ECHO BLOCK] {text[:100]}"
            if "Detected tool invocation loop" in text:
                return f"id={rid} [GRAPH BLOCK] {text[:100]}"
            if "CRITICAL REASONING ALERT" in text:
                return f"id={rid} [SEMANTIC ALERT] {text[:120]}..."
            return f"id={rid} {text[:120]}"
        if obj.get("error"):
            return f"id={rid} error: {obj['error']}"
        return f"id={rid} {json.dumps(result)[:120]}"
