"""Diagnose Codex metadata lookup using scratch configuration and local replies."""

import argparse
import http.server
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import threading


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--catalog", help="Optional diagnostic model_catalog_json")
    parser.add_argument("--model", default="moonshotai/Kimi-K3")
    parser.add_argument("--context-window", type=int)
    options = parser.parse_args()
    client = shutil.which("codex")
    if client is None:
        raise SystemExit("Codex must already be installed")
    requests = []

    class Fixture(http.server.BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            requests.append(json.loads(self.rfile.read(int(self.headers["Content-Length"]))))
            message = {
                "id": "msg_fixture", "type": "message", "role": "assistant",
                "status": "completed",
                "content": [{"type": "output_text", "text": "Fixture complete.", "annotations": []}],
            }
            response = {
                "id": "resp_fixture", "object": "response", "status": "completed",
                "model": options.model, "output": [message],
                "usage": {"input_tokens": 10, "output_tokens": 2, "total_tokens": 12},
            }
            events = [
                ("response.created", {"response": dict(response, status="in_progress", output=[])}),
                ("response.output_item.added", {"output_index": 0, "item": message}),
                ("response.output_item.done", {"output_index": 0, "item": message}),
                ("response.completed", {"response": response}),
            ]
            data = "".join(
                "event: " + name + "\ndata: " + json.dumps(dict(body, type=name)) + "\n\n"
                for name, body in events
            ).encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="tofa-metadata-loop-") as root:
            client_home = pathlib.Path(root) / "codex"
            client_home.mkdir()
            environment = {
                name: value for name, value in os.environ.items()
                if name in ("PATH", "TMPDIR", "SYSTEMROOT", "WINDIR")
            }
            environment.update(
                HOME=root, CODEX_HOME=str(client_home),
                TOFA_FIXTURE_KEY="synthetic-only", OTEL_SDK_DISABLED="true",
            )
            provider = (
                '{ name="Fixture", base_url="http://127.0.0.1:' + str(server.server_port)
                + '/v1", env_key="TOFA_FIXTURE_KEY", wire_api="responses", '
                'requires_openai_auth=false, supports_websockets=false, '
                'request_max_retries=0, stream_max_retries=0 }'
            )
            command = [
                client, "-c", 'model_provider="fixture"',
                "-c", "model=" + json.dumps(options.model),
                "-c", "model_providers.fixture=" + provider,
                "-c", 'web_search="disabled"',
            ]
            if options.catalog:
                command.extend(["-c", "model_catalog_json=" + json.dumps(str(pathlib.Path(options.catalog).resolve()))])
            if options.context_window is not None:
                command.extend(["-c", "model_context_window=" + str(options.context_window)])
            result = subprocess.run(
                command + ["exec", "--skip-git-repo-check", "--json", "Reply briefly."],
                cwd=root, env=environment, capture_output=True, text=True, timeout=30,
            )
            output = result.stdout + result.stderr
            lines = [line for line in output.splitlines() if "metadata" in line.lower()]
            print("client_exit:", result.returncode, "local_requests:", len(requests))
            print("\n".join(lines))
            if result.returncode != 0 or len(requests) != 1:
                print(output[-2000:])
                return 2
            if lines:
                print("REPRODUCED: unknown-model metadata warning")
                return 1
            print("PASS: no metadata warning")
            return 0
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    raise SystemExit(main())
