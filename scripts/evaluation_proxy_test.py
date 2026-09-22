"""Local HTTP seam tests; synthetic credentials and model output only."""
import http.server
import json
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request

from evaluation_proxy import EvaluationProxy, BODY_LIMIT


class ProxyTests(unittest.TestCase):
    def test_guardian_decision_is_measured_separately_from_synthetic_task(self):
        seen = []
        class Upstream(http.server.BaseHTTPRequestHandler):
            def log_message(self, *unused): pass
            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                seen.append(body)
                self.send_response(200)
                self.end_headers()
                for event in [
                    {'type': 'response.output_text.delta', 'delta': '{"out'},
                    {'type': 'response.output_text.delta', 'delta': 'come":"deny","rationale":"PRIVATE"}'},
                    {'type': 'response.completed'}]:
                    self.wfile.write(('data: ' + json.dumps(event) + '\n\n').encode())
        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Upstream)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                budget = root / 'budget.json'
                budget.write_text('{"used":0,"maximum":8}')
                report = root / 'observations.json'
                with EvaluationProxy('http://127.0.0.1:' + str(server.server_port), 'fixture-token',
                                     report, budget, 'deny', 10) as proxy:
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                    def post(body):
                        request = urllib.request.Request(proxy.url + '/responses', data=json.dumps({'model': 'moonshotai/Kimi-K3', 'stream': True, 'input': [], **body}).encode(),
                            headers={'Authorization': 'Bearer fixture-token'})
                        return opener.open(request).read()
                    ordinary = post({'input': []})
                    self.assertIn(b'function_call', ordinary)
                    post({'text': {'format': {'name': 'guardian_assessment', 'type': 'json_schema'}},
                          'instructions': 'Preserve this policy', 'tools': [{'name': 'exec_command'}]})
                self.assertEqual(len(seen), 1)
                self.assertEqual(seen[0]['max_output_tokens'], 4096)
                self.assertEqual(seen[0]['instructions'], 'Preserve this policy')
                self.assertEqual(json.loads(budget.read_text())['used'], 1)
                data = json.loads(report.read_text())
                self.assertEqual(data[-1]['kind'], 'automatic_review')
                self.assertEqual(data[-1]['decision'], 'deny')
                self.assertNotIn('PRIVATE', report.read_text())
        finally:
            server.shutdown()
            server.server_close()

    def test_budget_and_body_limits_stop_before_upstream(self):
        for body, used, expected_status, failure in [
                ({'input': []}, 0, 413, 'request_body_limit'),
                ({'input': []}, 8, 429, 'request_budget_limit')]:
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                budget = root / 'budget.json'
                budget.write_text(json.dumps({'used': used, 'maximum': 8}))
                output = root / 'observations.json'
                with EvaluationProxy('http://127.0.0.1:1', 'fixture-token', output,
                                     budget, 'coding', 1) as proxy:
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                    request = urllib.request.Request(proxy.url + '/responses', data=json.dumps({'model': 'moonshotai/Kimi-K3', 'stream': True, 'input': [], **body}).encode(),
                        headers={'Authorization': 'Bearer fixture-token'})
                    if failure == 'request_body_limit':
                        request.add_header('Content-Length', str(BODY_LIMIT + 1))
                    with self.assertRaises(urllib.error.HTTPError) as error:
                        opener.open(request)
                    self.assertEqual(error.exception.code, expected_status)
                records = json.loads(output.read_text())
                self.assertEqual(records[0]['failure'], failure)
                self.assertFalse(records[0]['completed'])
                self.assertEqual(records[0]['status'], expected_status)
                self.assertEqual(json.loads(budget.read_text())['used'], used)

    def test_unknown_models_server_context_and_nontext_inputs_are_refused(self):
        for change in ({'model': 'other/model'}, {'stream': False},
                       {'previous_response_id': 'synthetic-server-history'},
                       {'input': [{'type': 'input_image', 'image_url': 'http://example.invalid'}]},
                       {'tools': [{'type': 'web_search'}]}):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                budget = root / 'budget.json'
                budget.write_text('{"used":0,"maximum":8}')
                report = root / 'observations.json'
                with EvaluationProxy('http://127.0.0.1:1', 'fixture-token', report, budget, 'coding', 1) as proxy:
                    body = {'model': 'moonshotai/Kimi-K3', 'stream': True, 'input': [], **change}
                    request = urllib.request.Request(proxy.url + '/responses', data=json.dumps(body).encode(),
                        headers={'Authorization': 'Bearer fixture-token'})
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                    with self.assertRaises(urllib.error.HTTPError) as error:
                        opener.open(request)
                    self.assertEqual(error.exception.code, 400)
                self.assertEqual(json.loads(budget.read_text())['used'], 0)
                self.assertEqual(json.loads(report.read_text())[0]['failure'], 'unsupported_request_contract')
                self.assertNotIn('example.invalid', report.read_text())

    def test_trickling_stream_cannot_extend_the_total_request_deadline(self):
        class Upstream(http.server.BaseHTTPRequestHandler):
            def log_message(self, *unused): pass
            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']))
                self.send_response(200); self.end_headers()
                try:
                    for _ in range(30):
                        self.wfile.write(b':'); self.wfile.flush(); time.sleep(0.1)
                except OSError: pass
        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Upstream)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                budget = root / 'budget.json'; budget.write_text('{"used":0,"maximum":8}')
                report = root / 'observations.json'
                with EvaluationProxy('http://127.0.0.1:' + str(server.server_port), 'fixture-token',
                                     report, budget, 'coding', 1) as proxy:
                    request = urllib.request.Request(proxy.url + '/responses',
                        data=b'{"model":"moonshotai/Kimi-K3","stream":true,"input":[]}',
                        headers={'Authorization': 'Bearer fixture-token'})
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                    start = time.monotonic()
                    opener.open(request, timeout=5).read()
                    elapsed = time.monotonic() - start
                self.assertLess(elapsed, 2)
                self.assertEqual(json.loads(report.read_text())[0]['failure'], 'deadline_incomplete')
        finally:
            server.shutdown(); server.server_close()

    def test_complete_oversized_event_is_rejected_before_json_parsing(self):
        class Upstream(http.server.BaseHTTPRequestHandler):
            def log_message(self, *unused): pass
            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']))
                self.send_response(200); self.end_headers()
                line = ('data: ' + json.dumps({'type': 'response.output_text.delta',
                                              'delta': 'x' * 263000}) + '\n\n').encode()
                self.wfile.write(line)
                self.wfile.write(b'data: {"type":"response.completed"}\n\n')
        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Upstream)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                budget = root / 'budget.json'; budget.write_text('{"used":0,"maximum":8}')
                report = root / 'observations.json'
                with EvaluationProxy('http://127.0.0.1:' + str(server.server_port), 'fixture-token',
                                     report, budget, 'coding', 5) as proxy:
                    request = urllib.request.Request(proxy.url + '/responses',
                        data=b'{"model":"moonshotai/Kimi-K3","stream":true,"input":[]}',
                        headers={'Authorization': 'Bearer fixture-token'})
                    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
                    opener.open(request).read()
                observed = json.loads(report.read_text())[0]
                self.assertEqual(observed['failure'], 'event_body_limit')
                self.assertFalse(observed['completed'])
                self.assertEqual(observed['text_deltas'], 0)
        finally:
            server.shutdown(); server.server_close()


if __name__ == '__main__': unittest.main()
