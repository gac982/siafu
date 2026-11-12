#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors

"""PTY smoke test for the interactive TUI.

It starts the demo server on an isolated port, runs the TUI under a real
pseudo-terminal, and verifies that keys work immediately (before the old
5-second terminal-color query) and that Q/Ctrl+D exit cleanly.
"""

import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PORT = 18080
BASE = f"http://127.0.0.1:{PORT}"


def wait_for_server() -> None:
    for _ in range(50):
        try:
            with urllib.request.urlopen(BASE + "/", timeout=0.2):
                return
        except Exception:
            time.sleep(0.1)
    raise RuntimeError("demo server did not start")


def run_tui(config_path: str) -> int:
    pid, master = pty.fork()
    if pid == 0:
        os.execv(
            os.path.join(ROOT, "bin", "siafu"),
            [os.path.join(ROOT, "bin", "siafu"), "run", "--no-save", config_path],
        )
        os._exit(127)

    fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
    output = b""
    start = time.time()
    sent = 0
    exit_code = None
    keys = [(1.0, b"x"), (1.6, b"x"), (2.2, b"\x04"), (4.0, b"q")]

    try:
        while True:
            elapsed = time.time() - start
            while sent < len(keys) and elapsed >= keys[sent][0]:
                os.write(master, keys[sent][1])
                sent += 1
            ready, _, _ = select.select([master], [], [], 0.05)
            if ready:
                try:
                    chunk = os.read(master, 65536)
                    if not chunk:
                        break
                    output += chunk
                except OSError:
                    break
            done, status = os.waitpid(pid, os.WNOHANG)
            if done:
                exit_code = os.waitstatus_to_exitcode(status)
                break
            if elapsed > 12:
                os.kill(pid, signal.SIGKILL)
                break
    finally:
        try:
            os.close(master)
        except OSError:
            pass

    if exit_code is None:
        _, status = os.waitpid(pid, 0)
        exit_code = os.waitstatus_to_exitcode(status)

    text = output.decode("utf-8", "ignore")
    if "EVENTOS" not in text:
        print("TUI smoke failed: X did not open the log view at ~1s", file=sys.stderr)
        return 1
    if "ABORTADA" not in text:
        print("TUI smoke failed: Ctrl+D did not produce an ABORTED report", file=sys.stderr)
        return 1
    if exit_code != 130:
        print(f"TUI smoke failed: exit={exit_code}, want 130", file=sys.stderr)
        return 1
    return 0


def main() -> int:
    binary = os.path.join(ROOT, "bin", "siafu")
    demo = os.path.join(ROOT, "bin", "siafu-demo")
    if not os.path.exists(binary) or not os.path.exists(demo):
        print("build first: make build demo-build", file=sys.stderr)
        return 2

    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as f:
        config = f.name
        with open(os.path.join(ROOT, "examples", "demo.yaml")) as src:
            f.write(src.read().replace("http://127.0.0.1:8080", BASE))

    server = subprocess.Popen(
        [demo, "-addr", f"127.0.0.1:{PORT}"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        wait_for_server()
        return run_tui(config)
    finally:
        server.terminate()
        try:
            server.wait(timeout=2)
        except subprocess.TimeoutExpired:
            server.kill()
        os.unlink(config)


if __name__ == "__main__":
    sys.exit(main())
