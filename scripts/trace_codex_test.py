"""Offline HTTP-boundary tests for the opt-in sanitized trace observer."""
import http.client
import http.server
import json
import os
import subprocess
import sys
from pathlib import Path
import tempfile
import threading
import time
import unittest

from trace_codex import TraceProxy


class TraceTests(unittest.TestCase):
    def test_forwards_stream_unchanged_and_only_records_safe_metadata(self):
        payload = json.dumps({
            'model': 'moonshotai/Kimi-K3', 'tools': [{'type': 'function', 'name': 'exec_command',
            'description': 'PRIVATE_DESCRIPTION', 'parameters': {'PRIVATE_SCHEMA': 'secret'}}],
            'text': {'format': {'type': 'json_schema', 'strict': False, 'schema': {'PRIVATE_SCHEMA': 1}}},
            'tool_choice': 'auto', 'input': [{'type': 'message', 'role': 'user', 'content': 'PRIVATE_PROMPT'}],
        }).encode()
        stream = b'data: {"type":"response.output_text.delta","delta":"Cannot combine tool calls with constrained decoding PRIVATE_OUTPUT"}\n\ndata: {"type":"response.completed","response":{"id":"PRIVATE_SESSION"}}\n\n'
        seen = []

        class Backend(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_POST(self):
                seen.append((self.path, self.headers['Authorization'], self.rfile.read(int(self.headers['Content-Length']))))
                self.send_response(200)
                self.send_header('Content-Type', 'text/event-stream')
                self.end_headers()
                self.wfile.write(stream)

        backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Backend)
        threading.Thread(target=backend.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / 'trace.jsonl'
                with TraceProxy(f'http://127.0.0.1:{backend.server_port}', 'PRIVATE_TOKEN', path) as proxy:
                    client = http.client.HTTPConnection('127.0.0.1', proxy.port, timeout=3)
                    client.request('POST', '/responses', payload, {'Authorization': 'Bearer PRIVATE_TOKEN'})
                    response = client.getresponse()
                    self.assertEqual(response.status, 200)
                    self.assertEqual(response.read(), stream)
                    client.close()
                self.assertEqual(seen, [('/responses', 'Bearer PRIVATE_TOKEN', payload)])
                raw = path.read_text()
                self.assertNotIn('PRIVATE_', raw)
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
                records = [json.loads(line) for line in raw.splitlines()]
                self.assertEqual(records[0]['request']['tools'], [{'type': 'function', 'name': 'exec_command'}])
                self.assertEqual(records[0]['request']['text_format'], {'type': 'json_schema', 'strict': False})
                self.assertEqual(records[-1]['events'], {'response.output_text.delta': 1, 'response.completed': 1})
                self.assertNotIn('error_class', records[-1])
        finally:
            backend.shutdown()
            backend.server_close()


    def test_nested_reviewer_tools_and_http_error_are_sanitized_without_mutation(self):
        payload = json.dumps({'model': 'codex-auto-review', 'input': [{'type': 'additional_tools', 'tools': [{'type': 'namespace', 'name': 'functions', 'description': 'PRIVATE_DESCRIPTION',
                                  'tools': [{'type': 'function', 'name': 'view_image', 'parameters': {'PRIVATE_SCHEMA': 1}}]}]},
            {'type': 'function_call_output', 'output': 'PRIVATE_RESULT'}],
            'tool_choice': {'type': 'function', 'name': 'PRIVATE_NAME'},
            'text': {'format': {'type': 'json_schema', 'strict': False}}}).encode()
        error = b'{"error":{"message":"Cannot combine tool calls with constrained decoding PRIVATE_ERROR","code":400}}'
        seen = []
        class Backend(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_POST(self):
                seen.append(self.rfile.read(int(self.headers['Content-Length'])))
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(error)
        backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Backend)
        threading.Thread(target=backend.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / 'trace.jsonl'
                with TraceProxy(f'http://127.0.0.1:{backend.server_port}', 'PRIVATE_TOKEN', path, max_requests=1) as proxy:
                    def request(route, token, body=payload):
                        client = http.client.HTTPConnection('127.0.0.1', proxy.port, timeout=3)
                        client.request('POST', route, body, {'Authorization': 'Bearer ' + token})
                        response = client.getresponse()
                        result = response.status, response.read()
                        client.close()
                        return result
                    self.assertEqual(request('/responses', 'wrong')[0], 401)
                    self.assertEqual(request('/other', 'PRIVATE_TOKEN')[0], 404)
                    self.assertEqual(request('/responses', 'PRIVATE_TOKEN'), (400, error))
                    self.assertEqual(request('/responses', 'PRIVATE_TOKEN')[0], 429)
                self.assertEqual(seen, [payload])
                raw = path.read_text()
                self.assertNotIn('PRIVATE_', raw)
                records = [json.loads(line) for line in raw.splitlines()]
                self.assertEqual(records[0]['request']['tools'], [
                    {'type': 'namespace', 'name': 'functions', 'source': 'input.additional_tools'},
                    {'type': 'function', 'name': 'view_image', 'source': 'namespace'}])
                self.assertEqual(records[0]['request']['tool_choice'], 'function')
                self.assertEqual(records[-1]['error_class'], 'tools_schema_conflict')
                self.assertEqual(records[-1]['status'], 400)
                with self.assertRaises(FileExistsError):
                    TraceProxy(f'http://127.0.0.1:{backend.server_port}', 'token', path)
        finally:
            backend.shutdown()
            backend.server_close()

    def test_cli_uses_real_launcher_injected_provider_with_isolated_client_home(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            normal_home = root / 'normal-codex'
            normal_home.mkdir()
            (normal_home / 'auth.json').write_text('PRIVATE_SAVED_AUTH')
            (normal_home / 'config.toml').write_text('PRIVATE_SAVED_CONFIG')
            launcher = root / 'tofa'
            client = root / 'actual-codex'
            launcher.write_text('#!' + sys.executable + '\n' +
                "import os,subprocess,sys\n"
                "os.environ['TOFA_API_KEY']='PRIVATE_TOKEN'\n"
                "args=sys.argv[sys.argv.index('--')+1:]\n"
                "sys.exit(subprocess.call(['codex','-c','model_providers.nebius-tofa={base_url=\"http://127.0.0.1:1\"}']+args))\n")
            client.write_text('#!' + sys.executable + '\n' +
                "import os,pathlib,sys\n"
                "assert os.environ['HOME'] != " + repr(str(root)) + "\n"
                "assert pathlib.Path(os.environ['CODEX_HOME']).is_dir()\n"
                "assert not (pathlib.Path(os.environ['CODEX_HOME'])/'auth.json').exists()\n"
                "assert 'PRIVATE_UPSTREAM_KEY' not in os.environ\n"
                "assert os.environ['TOFA_API_KEY']=='PRIVATE_TOKEN'\n"
                "assert '--approve-for-me' in sys.argv\n"
                "assert '--sandbox' not in sys.argv\n"
                "assert 'read-only' not in sys.argv\n"
                "print('PRIVATE_STDOUT')\n"
                "print('PRIVATE_STDERR',file=sys.stderr)\n")
            launcher.chmod(0o700)
            client.chmod(0o700)
            trace = root / 'trace.jsonl'
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('trace_codex.py')),
                '--launcher', str(launcher), '--codex', str(client), '--output', str(trace),
                '--approval-probe', '--timeout', '10'], env=dict(os.environ, HOME=str(root), CODEX_HOME=str(normal_home), PRIVATE_UPSTREAM_KEY='PRIVATE_SECRET'),
                text=True, capture_output=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            self.assertNotIn('PRIVATE_', result.stdout + result.stderr + trace.read_text())
            self.assertEqual((normal_home / 'auth.json').read_text(), 'PRIVATE_SAVED_AUTH')
            self.assertEqual((normal_home / 'config.toml').read_text(), 'PRIVATE_SAVED_CONFIG')

    def test_stream_flushes_before_backend_finishes_and_sse_error_is_classified(self):
        first = b'data: {"type":"response.created","response":{"id":"PRIVATE_SESSION"}}\n\n'
        last = b'data: {"type":"error","message":"Cannot combine tool calls with constrained decoding PRIVATE_ERROR"}\n\ndata: [DONE]\n\n'
        release = threading.Event()
        class Backend(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']))
                self.send_response(200)
                self.send_header('Content-Type', 'text/event-stream')
                self.end_headers()
                self.wfile.write(first)
                self.wfile.flush()
                release.wait(3)
                self.wfile.write(last)
        backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Backend)
        threading.Thread(target=backend.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / 'trace.jsonl'
                with TraceProxy(f'http://127.0.0.1:{backend.server_port}', 'token', path) as proxy:
                    client = http.client.HTTPConnection('127.0.0.1', proxy.port, timeout=1)
                    client.request('POST', '/responses', b'{"model":"PRIVATE_MODEL","tool_choice":"PRIVATE_CHOICE"}',
                                   {'Authorization': 'Bearer token'})
                    response = client.getresponse()
                    self.assertEqual(response.read(len(first)), first)
                    self.assertFalse(release.is_set())
                    self.assertIn('"phase":"request"', path.read_text())
                    release.set()
                    self.assertEqual(response.read(), last)
                    client.close()
                raw = path.read_text()
                self.assertNotIn('PRIVATE_', raw)
                records = [json.loads(line) for line in raw.splitlines()]
                self.assertEqual(records[-1]['events'], {'response.created': 1, 'error': 1, 'done_marker': 1})
                self.assertEqual(records[-1]['error_class'], 'tools_schema_conflict')
        finally:
            release.set()
            backend.shutdown()
            backend.server_close()

    def test_header_deadline_records_stage_without_private_exception_text(self):
        class Backend(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']))
                time.sleep(2)
        backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Backend)
        threading.Thread(target=backend.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / 'trace.jsonl'
                with TraceProxy(f'http://127.0.0.1:{backend.server_port}', 'token', path, timeout=1) as proxy:
                    client = http.client.HTTPConnection('127.0.0.1', proxy.port, timeout=3)
                    client.request('POST', '/responses', b'{}', {'Authorization': 'Bearer token'})
                    response = client.getresponse()
                    self.assertEqual(response.status, 502)
                    response.read()
                    client.close()
                record = json.loads(path.read_text().splitlines()[-1])
                self.assertEqual(record['error'], 'timeout')
                self.assertEqual(record['stage'], 'response_headers')
                self.assertGreaterEqual(record['elapsed_ms'], 900)
                self.assertLess(record['elapsed_ms'], 2000)
        finally:
            backend.shutdown()
            backend.server_close()

    def test_refuses_nonlocal_or_ambiguous_adapter_targets(self):
        with tempfile.TemporaryDirectory() as directory:
            for endpoint in ('https://127.0.0.1:80', 'http://localhost:80', 'http://example.com',
                             'http://127.0.0.1:80/responses', 'http://user@127.0.0.1:80',
                             'http://127.0.0.1:80?private=1'):
                with self.assertRaises(ValueError):
                    TraceProxy(endpoint, 'token', Path(directory) / 'trace.jsonl')


if __name__ == '__main__':
    unittest.main()
