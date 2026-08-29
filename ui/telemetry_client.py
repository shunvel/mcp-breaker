"""Unix-socket telemetry client for mcp-breaker wrap sessions."""

from __future__ import annotations

import json
import queue
import socket
import threading
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class DashboardState:
    events: list[str] = field(default_factory=list)
    metrics: dict[str, Any] = field(default_factory=dict)
    paused: bool = False
    pattern: str = ""
    last_sim: float = 0.0
    max_events: int = 50

    def apply(self, ev: dict[str, Any]) -> None:
        line = f"[{ev.get('type', '?')}]"
        if ev.get("method"):
            line += f" {ev['method']}"
        if ev.get("tool"):
            line += f" {ev['tool']}"
        if ev.get("pattern"):
            line += f" {ev['pattern']}"
        if ev.get("similarity"):
            self.last_sim = float(ev["similarity"])
            line += f" sim={self.last_sim:.3f}"
        self.events.append(line)
        if len(self.events) > self.max_events:
            self.events = self.events[-self.max_events :]

        if ev.get("metrics"):
            self.metrics = dict(ev["metrics"])

        ev_type = ev.get("type")
        if ev_type in ("paused", "graph_trip"):
            self.paused = True
            self.pattern = ev.get("pattern", "")
        elif ev_type == "resumed":
            self.paused = False
            self.pattern = ""


def drain_event_queue(event_queue: queue.Queue, state: DashboardState) -> int:
    """Apply queued events on the main Streamlit thread (thread-safe)."""
    count = 0
    while True:
        try:
            ev = event_queue.get_nowait()
        except queue.Empty:
            break
        state.apply(ev)
        count += 1
    return count


class EventListener:
    """Listens on the wrap event socket (same role as `mcp-breaker dashboard`)."""

    def __init__(self, socket_path: Path, event_queue: queue.Queue) -> None:
        self.socket_path = socket_path
        self.event_queue = event_queue
        self._server: socket.socket | None = None
        self._thread: threading.Thread | None = None
        self._stop = threading.Event()
        self.last_error: str | None = None

    @property
    def active(self) -> bool:
        return self._server is not None and not self._stop.is_set()

    def start(self) -> None:
        self.stop()
        self._stop.clear()
        self.last_error = None
        self.socket_path.parent.mkdir(parents=True, exist_ok=True)
        if self.socket_path.exists():
            try:
                self.socket_path.unlink()
            except OSError as exc:
                self.last_error = f"cannot remove stale socket: {exc}"
                return

        try:
            self._server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            self._server.bind(str(self.socket_path))
            self._server.listen(8)
        except OSError as exc:
            self.last_error = f"cannot bind event socket: {exc}"
            self._server = None
            return

        self._thread = threading.Thread(target=self._serve, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        if self._server:
            try:
                self._server.close()
            except OSError:
                pass
            self._server = None
        if self.socket_path.exists():
            try:
                self.socket_path.unlink()
            except OSError:
                pass

    def _serve(self) -> None:
        assert self._server is not None
        while not self._stop.is_set():
            try:
                self._server.settimeout(0.5)
                conn, _ = self._server.accept()
            except TimeoutError:
                continue
            except OSError:
                break
            threading.Thread(target=self._handle_conn, args=(conn,), daemon=True).start()

    def _handle_conn(self, conn: socket.socket) -> None:
        with conn:
            buf = b""
            while not self._stop.is_set():
                try:
                    chunk = conn.recv(4096)
                except OSError:
                    break
                if not chunk:
                    break
                buf += chunk
                while b"\n" in buf:
                    line, buf = buf.split(b"\n", 1)
                    if not line.strip():
                        continue
                    try:
                        ev = json.loads(line)
                        self.event_queue.put(ev)
                    except json.JSONDecodeError:
                        continue


def send_resume(control_socket: Path, session_id: str = "") -> None:
    payload = json.dumps({"action": "resume", "session_id": session_id}).encode()
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as conn:
        conn.connect(str(control_socket))
        conn.sendall(payload)


def default_run_dir() -> Path:
    return Path.home() / ".mcp-breaker" / "run"
