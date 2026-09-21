"""Offline executable-boundary tests; never reads saved credentials or calls inference."""
import http.server
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
import unittest

HARNESS = Path(__file__).with_name("live_compat.py")


class LiveCompatibilityTests(unittest.TestCase):
    def fixture(self, mode="success", runs=1, timeout=10):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            launcher = root / "tofa"
            client = root / "real-codex"
            normal_client = root / "normal-codex"
            normal_client.mkdir()
            normal_auth = normal_client / "auth.json"
            normal_auth.write_text('{"fixture":"before"}')
            if mode == "no_dns":
                (root / "sitecustomize.py").write_text(
                    "import socket, sys\n"
                    "if '--observe' in sys.argv:\n"
                    "    def unavailable(*args):\n"
                    "        raise RuntimeError('loopback listener must not require reverse DNS')\n"
                    "    socket.getfqdn = unavailable\n")
            if mode == "coarse_clock":
                (root / "sitecustomize.py").write_text(
                    "import sys, time\n"
                    "if '--observe' in sys.argv:\n"
                    "    time.monotonic = lambda: 1234.0\n")
            launcher.write_text("#!" + sys.executable + "\n" + '''
import json, os, subprocess, sys
if sys.argv[1:] == ['--version']:
    print('tofa fixture'); sys.exit()
args = sys.argv[sys.argv.index('--')+1:]
provider = 'model_providers.nebius-tofa={base_url='+json.dumps(os.environ['FIXTURE_ENDPOINT'])+'}'
os.environ['TOFA_API_KEY'] = 'local-fixture-token'
sys.exit(subprocess.call(['codex', '-c', provider] + args))
''')
            client.write_text("#!" + sys.executable + "\nmode = " + repr(mode)
                              + "\nnormal_auth = " + repr(str(normal_auth)) + "\n" + '''
import json, os, pathlib, re, sys, urllib.request
if sys.argv[1:] == ['--version']:
    print('codex-cli fixture'); sys.exit()
if mode == 'timeout':
    import time
    time.sleep(30)
if mode == 'trust_workspace':
    config = pathlib.Path(os.environ['CODEX_HOME'])/'config.toml'
    trust = '[projects.'+json.dumps(str(pathlib.Path.cwd()))+']\\ntrust_level = "trusted"\\n'
    if trust not in config.read_text(): config.write_text(config.read_text()+'\\n'+trust)
if mode == 'normal_auth_changed': pathlib.Path(normal_auth).write_text('{"fixture":"after"}')
provider = next(a for a in sys.argv if a.startswith('model_providers.'))
endpoint = json.loads(re.search(r'base_url\\s*=\\s*("[^"]+")', provider).group(1))
request = urllib.request.Request(endpoint+'/responses', data=b'{}', headers={'Authorization':'Bearer '+os.environ['TOFA_API_KEY']})
# This fixture must never consult system proxies for its loopback-only requests.
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
with opener.open(request, timeout=5) as response:
    if mode == 'client_closes_completed':
        while True:
            line = response.readline()
            if b'response.completed' in line: break
    else: response.read()
continued = 'resume' in sys.argv
values = {'count':4,'total':18,'max':9}
if continued: values.update(min=-2, average=4.5)
if mode == 'wrong_files': values['total'] = 999
pathlib.Path('summary.json').write_text(json.dumps(values))
identity = '11111111-1111-4111-8111-111111111111'
if mode == 'wrong_session' and continued: identity = '22222222-2222-4222-8222-222222222222'
print(json.dumps({'type':'thread.started','thread_id':identity}))
print(json.dumps({'type':'item.completed','item':{'type':'command_execution','status':'completed','exit_code':1 if mode == 'failed_tool' else 0,'aggregated_output':'PRIVATE_BODY'}}))
print(json.dumps({'type':'turn.completed'}))
if mode == 'metadata_warning': print('Model metadata for PRIVATE_BODY', file=sys.stderr)
''')
            launcher.chmod(0o700)
            client.chmod(0o700)
            seen = []

            class Upstream(http.server.BaseHTTPRequestHandler):
                def log_message(self, *args):
                    pass

                def do_POST(self):
                    self.rfile.read(int(self.headers["Content-Length"]))
                    seen.append(self.headers["Authorization"])
                    self.send_response(200)
                    self.send_header("Content-Type", "text/event-stream")
                    self.end_headers()
                    kinds = ["response.completed"] if mode == "no_streaming" else ["response.output_text.delta", "response.output_text.delta", "response.completed"]
                    for kind in kinds:
                        self.wfile.write(("data: " + json.dumps({"type": kind, "delta": "" if mode == "empty_deltas" else "PRIVATE_BODY"}) + "\n\n").encode())
                        self.wfile.flush()
                    if mode == "client_closes_completed":
                        time.sleep(0.1)
                        try:
                            for _ in range(100):
                                self.wfile.write(b":" + b"keepalive " * 10000 + b"\n\n")
                                self.wfile.flush()
                        except OSError:
                            pass

            server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Upstream)
            threading.Thread(target=server.serve_forever, daemon=True).start()
            try:
                env = {**os.environ, "HOME": str(root), "CODEX_HOME": str(root / "normal-codex"),
                       "FIXTURE_ENDPOINT": f"http://127.0.0.1:{server.server_port}"}
                if mode in ("no_dns", "coarse_clock"):
                    env["PYTHONPATH"] = str(root)
                report = root / "evidence.json"
                result = subprocess.run([sys.executable, str(HARNESS), "--launcher", str(launcher),
                                         "--codex", str(client), "--runs", str(runs), "--timeout", str(timeout), "--output", str(report)],
                                        env=env, capture_output=True, text=True, timeout=90)
                evidence = json.loads(report.read_text())
                self.assertEqual(len(evidence["runs"]), runs)
                if mode != "timeout":
                    self.assertEqual(len(seen), runs * 2, json.dumps(evidence))
                    self.assertEqual(set(seen), {"Bearer local-fixture-token"})
                self.assertNotIn("PRIVATE_BODY", report.read_text())
                self.assertNotIn("local-fixture-token", report.read_text())
                return result, evidence
            finally:
                server.shutdown()
                server.server_close()

    def test_three_runs_require_tools_files_continuation_and_streaming(self):
        result, evidence = self.fixture(runs=3)
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertTrue(evidence["passed"])
        self.assertTrue(all(run["passed"] for run in evidence["runs"]))

    def test_rejects_failed_tool_despite_correct_files(self):
        result, evidence = self.fixture("failed_tool")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(evidence["passed"])

    def test_rejects_incorrect_files_despite_successful_tool(self):
        result, evidence = self.fixture("wrong_files")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(evidence["passed"])

    def test_rejects_new_session_instead_of_continuation(self):
        result, evidence = self.fixture("wrong_session")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(evidence["runs"][0]["same_session"])

    def test_rejects_completed_response_without_streaming(self):
        result, evidence = self.fixture("no_streaming")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(evidence["runs"][0]["turns"][0]["streaming_observed"])

    def test_rejects_metadata_warning(self):
        result, evidence = self.fixture("metadata_warning")
        self.assertEqual(result.returncode, 1)
        self.assertTrue(evidence["runs"][0]["turns"][0]["metadata_warning"])

    def test_empty_deltas_do_not_prove_streaming(self):
        result, evidence = self.fixture("empty_deltas")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(evidence["runs"][0]["turns"][0]["streaming_observed"])

    def test_client_may_close_after_completed_response(self):
        result, evidence = self.fixture("client_closes_completed")
        self.assertEqual(result.returncode, 0)
        self.assertTrue(evidence["passed"])

    def test_client_workspace_trust_is_predeclared_in_scratch_config(self):
        result, evidence = self.fixture("trust_workspace")
        self.assertEqual(result.returncode, 0)
        self.assertTrue(evidence["runs"][0]["scratch_settings_preserved"])

    def test_normal_auth_change_is_identified_without_exporting_its_content(self):
        result, evidence = self.fixture("normal_auth_changed")
        self.assertEqual(result.returncode, 1)
        self.assertFalse(evidence["normal_files_preserved"]["codex_auth"])
        self.assertTrue(evidence["normal_files_preserved"]["codex_config"])
        self.assertNotIn('"fixture"', json.dumps(evidence))

    def test_loopback_listener_does_not_require_reverse_dns(self):
        result, evidence = self.fixture("no_dns")
        self.assertEqual(result.returncode, 0)
        self.assertTrue(evidence["passed"])

    def test_stream_timing_uses_high_resolution_when_deadline_clock_is_coarse(self):
        result, evidence = self.fixture("coarse_clock")
        self.assertEqual(result.returncode, 0, json.dumps(evidence))
        self.assertTrue(evidence["passed"])

    def test_timeout_records_failure_without_retry(self):
        started = time.monotonic()
        result, evidence = self.fixture("timeout", timeout=1)
        self.assertEqual(result.returncode, 1)
        self.assertTrue(evidence["runs"][0]["turns"][0]["timed_out"])
        self.assertEqual(len(evidence["runs"][0]["turns"]), 1)
        self.assertLess(time.monotonic() - started, 10)


if __name__ == "__main__":
    unittest.main()
