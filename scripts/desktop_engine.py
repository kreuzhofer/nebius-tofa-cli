"""Stdio app-server client shared by desktop qualification and history checks."""
from collections import deque
import json
import os
import queue
import subprocess
import tempfile
import threading
import time


class Engine:
    def __init__(self, executable, root, overrides, environment=None):
        env = {key: os.environ[key] for key in ("PATH", "TMPDIR") if key in os.environ}
        env.update(HOME=str(root), CODEX_HOME=str(root / "codex"),
                   OTEL_SDK_DISABLED="true", TOFA_HISTORY_FIXTURE_KEY="synthetic")
        if environment is not None:
            env = environment
        args = [executable, "app-server"]
        for override in overrides:
            args.extend(["-c", override])
        self.errors = tempfile.TemporaryFile(mode="w+t")
        self.process = subprocess.Popen(
            args, cwd=root / "workspace", env=env, stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=self.errors, text=True,
        )
        self.messages = queue.Queue()
        self.sequence = 0
        self.pending = deque()
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()
        try:
            self.call("initialize", {"clientInfo": {"name": "tofa_history_fixture", "version": "1"},
                                     "capabilities": {"experimentalApi": True}})
            self.send({"method": "initialized"})
        except BaseException:
            self.close()
            raise

    def _read(self):
        try:
            for line in self.process.stdout:
                self.messages.put(json.loads(line))
        finally:
            self.messages.put(None)

    def send(self, message):
        self.process.stdin.write(json.dumps(message) + "\n")
        self.process.stdin.flush()

    def receive(self, deadline):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("fixture engine deadline exceeded")
        try:
            message = self.pending.popleft() if self.pending else self.messages.get(timeout=remaining)
        except queue.Empty as error:
            raise TimeoutError("fixture engine deadline exceeded") from error
        if message is None:
            self.errors.seek(0)
            raise RuntimeError("fixture engine exited before responding: " + self.errors.read())
        if "method" in message and "id" in message:
            raise RuntimeError("unexpected engine callback: " + message["method"])
        return message

    def response(self, method, params, deadline=None):
        if deadline is None:
            deadline = time.monotonic() + 30
        self.sequence += 1
        self.send({"id": self.sequence, "method": method, "params": params})
        notifications = []
        try:
            while True:
                message = self.receive(deadline)
                if message.get("id") == self.sequence:
                    return message
                notifications.append(message)
        finally:
            self.pending.extendleft(reversed(notifications))

    def call(self, method, params, deadline=None):
        message = self.response(method, params, deadline)
        if "error" in message:
            raise RuntimeError(method + ": " + message["error"]["message"])
        return message["result"]

    def turn(self, thread_id, text):
        deadline = time.monotonic() + 30
        self.call("turn/start", {"threadId": thread_id, "input": [{"type": "text", "text": text}]}, deadline)
        while True:
            message = self.receive(deadline)
            if message.get("method") == "turn/completed" and message["params"]["threadId"] == thread_id:
                turn = message["params"]["turn"]
                if turn["status"] != "completed":
                    raise RuntimeError("synthetic turn failed: " + json.dumps(turn.get("error")))
                return

    def close(self):
        self.process.stdin.close()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(timeout=5)
        self.reader.join(timeout=5)
        self.process.stdout.close()
        self.errors.close()
