"""Optional real Codex CLI seam against loopback-only synthetic model responses."""
import http.server
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

from evaluation_proxy import fixture_response, message
from live_compat import LoopbackServer


@unittest.skipUnless(os.environ.get('TOFA_TEST_CODEX'), 'set TOFA_TEST_CODEX for installed-client fixture')
class InstalledEvaluationTests(unittest.TestCase):
    def test_actual_guardian_decisions_control_benign_execution(self):
        seen = []
        class Upstream(http.server.BaseHTTPRequestHandler):
            def log_message(self, *unused): pass
            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                review = body.get('text', {}).get('format', {}).get('type') == 'json_schema'
                deny = 'prompt-injection fixture' in json.dumps(body)
                seen.append(('review' if review else 'coding', deny))
                text = json.dumps({'outcome': 'deny' if deny else 'allow'}) if review else 'Fixture complete.'
                self.send_response(200)
                self.send_header('Content-Type', 'text/event-stream')
                self.end_headers()
                delta = {'type': 'response.output_text.delta', 'delta': text}
                self.wfile.write(('data: ' + json.dumps(delta) + '\n\n').encode())
                self.wfile.write(fixture_response(message(text)))
        server = LoopbackServer(('127.0.0.1', 0), Upstream)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                launcher = root / 'launcher'
                launcher.write_text('#!' + sys.executable + '\n' + '''
import json, os, subprocess, sys
if sys.argv[1:] == ['--version']:
    print('tofa fixture'); sys.exit()
args = sys.argv[sys.argv.index('--')+1:]
if any(arg.startswith(('-c', '--config', '--model', '--profile')) for arg in args): sys.exit(2)
provider = 'model_providers.nebius-tofa={name="Fixture",base_url='+json.dumps(os.environ['FIXTURE_ENDPOINT'])+',env_key="TOFA_API_KEY",wire_api="responses",supports_websockets=false,request_max_retries=0,stream_max_retries=0}'
os.environ['TOFA_API_KEY'] = 'synthetic-fixture-token'
sys.exit(subprocess.call(['codex','-c',provider,'-c','model_provider="nebius-tofa"','--model','moonshotai/Kimi-K3','-c','web_search="disabled"','-c','model_catalog_json='+json.dumps(os.environ['FIXTURE_CATALOG'])]+args))
''')
                launcher.chmod(0o700)
                catalog = root / 'catalog.json'
                catalog.write_text(json.dumps({'models': [{
                    'slug': 'moonshotai/Kimi-K3', 'display_name': 'Fixture', 'description': 'Synthetic model',
                    'supported_reasoning_levels': [], 'shell_type': 'unified_exec', 'visibility': 'list',
                    'supported_in_api': True, 'priority': 0, 'include_apps_usage_instructions': False,
                    'supports_reasoning_summary_parameter': False, 'support_verbosity': False,
                    'truncation_policy': {'mode': 'bytes', 'limit': 10000},
                    'context_window': 1024000, 'max_context_window': 1024000,
                    'effective_context_window_percent': 95, 'experimental_supported_tools': [],
                    'input_modalities': ['text'],
                    'model_messages': {'instructions_template': 'Use tools as requested; obey the approval policy.'}
                }]}))
                output = root / 'report.json'
                env = dict(os.environ, HOME=str(root), CODEX_HOME=str(root / 'normal-codex'),
                           XDG_CONFIG_HOME=str(root / 'config'),
                           FIXTURE_ENDPOINT='http://127.0.0.1:' + str(server.server_port), FIXTURE_CATALOG=str(catalog))
                result = subprocess.run([sys.executable, str(Path(__file__).with_name('model_evaluation.py')),
                    '--launcher', str(launcher), '--codex', os.environ['TOFA_TEST_CODEX'],
                    '--output', str(output)], env=env, capture_output=True, text=True, timeout=120)
                self.assertEqual(result.returncode, 1, result.stderr)
                report = json.loads(output.read_text())
                cases = report['automatic_approval']['cases']
                self.assertEqual(len(cases), 6, json.dumps(report))
                for case in cases:
                    self.assertEqual(case['decisions'], [case['case']], json.dumps(case))
                    self.assertEqual(case['command_executed'], case['case'] == 'allow', json.dumps(case))
                    self.assertTrue(case['passed'], json.dumps(case))
                self.assertEqual(sum(kind == 'review' for kind, _ in seen), 6)
                self.assertTrue(report['normal_settings_preserved'])
                self.assertNotIn('synthetic-fixture-token', output.read_text())
        finally:
            server.shutdown(); server.server_close()


if __name__ == '__main__': unittest.main()
