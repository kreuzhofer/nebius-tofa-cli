"""Compiled launcher PTY checks; only synthetic credentials and loopback inference."""
import http.server
import json
import os
import pathlib
import pty
import select
import signal
import subprocess
import sys
import tempfile
import termios
import threading
import time
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
KIMI = 'moonshotai/Kimi-K3'
GLM = 'zai-org/GLM-5.3-Flash'
DEEPSEEK = 'deepseek-ai/DeepSeek-V4.1-Flash'


class PickerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build = tempfile.TemporaryDirectory()
        cls.binary = pathlib.Path(cls.build.name) / 'picker-launcher'
        subprocess.run(['go', 'build', '-o', str(cls.binary), './scripts/fixtures/picker_launcher'], cwd=ROOT, check=True)
        # Synthetic support records are compiled only into this test executable.
        # Production assets and their experimental status remain untouched.
        records = [
            dict(target='codex', route='adapted', main=KIMI, guardian=GLM),
            dict(target='codex', route='adapted', main=DEEPSEEK, guardian=KIMI),
            dict(target='codex', route='direct', main=DEEPSEEK, guardian=''),
        ]
        for record in records:
            record.update(status='supported', evidence='test-only synthetic verification')
        snapshot = pathlib.Path(cls.build.name) / 'verification.json'
        snapshot.write_text(json.dumps({'records': records}))
        overlay = pathlib.Path(cls.build.name) / 'overlay.json'
        overlay.write_text(json.dumps({'Replace': {str(ROOT / 'internal/tofa/assets/model-verification.json'): str(snapshot)}}))
        cls.supported = pathlib.Path(cls.build.name) / 'supported-launcher'
        subprocess.run(['go', 'build', '-overlay', str(overlay), '-o', str(cls.supported), './scripts/fixtures/picker_launcher'], cwd=ROOT, check=True)

    @classmethod
    def tearDownClass(cls):
        cls.build.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = pathlib.Path(self.temp.name)
        self.store = self.home / 'store'
        self.store.mkdir(mode=0o700)
        ref = 'a' * 32
        (self.store / 'config.yml').write_text(f'version: 1\nproject_id: synthetic-project\nmodel: {GLM}\ncredential_backend: file\ncredential_ref: {ref}\n')
        (self.store / 'credentials.yml').write_text(f'{ref}: synthetic-key\n')
        for path in self.store.iterdir():
            path.chmod(0o600)
        self.saved = {p.name: p.read_bytes() for p in self.store.iterdir()}
        self.models = [DEEPSEEK, 'mid/unknown-model', KIMI, GLM]
        self.requests = []
        self.catalog_status = 200
        owner = self

        class Provider(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                owner.requests.append((self.path, self.headers.get('Authorization'), None))
                self.send_response(owner.catalog_status)
                self.end_headers()
                self.wfile.write(json.dumps({'data': [{'id': m} for m in owner.models]}).encode())

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                owner.requests.append((self.path, self.headers.get('Authorization'), body))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b'{"output":[]}')

            def log_message(self, *_):
                pass

        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Provider)
        thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.marker = self.home / 'launched.json'
        client = self.home / 'codex'
        client.write_text('#!' + sys.executable + '\n' + '''import json,os,re,sys,urllib.request
args=sys.argv[1:]
model=json.loads(next(a[6:] for a in args if a.startswith('model=')))
provider=next(a for a in args if a.startswith('model_providers.nebius-tofa='))
endpoint=json.loads(re.search(r'base_url = ("[^"]+")',provider).group(1))
if endpoint.startswith('http://127.0.0.1:'):
 request=urllib.request.Request(endpoint+'/responses',json.dumps({'model':model,'input':[]}).encode(),{'Authorization':'Bearer '+os.environ['TOFA_API_KEY'],'Content-Type':'application/json'})
 with urllib.request.urlopen(request) as response: assert response.status==200
open(os.environ['FIXTURE_MARKER'],'w').write(json.dumps({'model':model,'args':args}))
''')
        client.chmod(0o700)
        self.env = dict(os.environ, HOME=str(self.home), XDG_CONFIG_HOME=str(self.home / '.config'),
                        PATH=str(self.home) + os.pathsep + os.environ['PATH'],
                        FIXTURE_ENDPOINT=f'http://127.0.0.1:{self.server.server_port}',
                        FIXTURE_DIR=str(self.store), FIXTURE_MARKER=str(self.marker))

    def start(self, args, binary=None):
        self.master, self.slave = pty.openpty()
        self.before = termios.tcgetattr(self.slave)
        self.process = subprocess.Popen([str(binary or self.binary), *args], stdin=self.slave, stdout=self.slave, stderr=self.slave, env=self.env)
        self.output = b''
        process, master, slave = self.process, self.master, self.slave
        def stop():
            if process.poll() is None:
                process.kill()
                process.wait()
            os.close(master)
            os.close(slave)
        self.addCleanup(stop)

    def read_until(self, text):
        target = text.encode()
        end = time.monotonic() + 5
        while target not in self.output and time.monotonic() < end:
            if select.select([self.master], [], [], .05)[0]:
                self.output += os.read(self.master, 65536)
        self.assertIn(target, self.output)
        return self.output.decode()

    def finish(self, code=0):
        self.assertEqual(self.process.wait(timeout=5), code)
        restored = termios.tcgetattr(self.slave)
        # macOS sets this transient kernel flag when tcsetattr re-enables ICANON.
        # It is not a changed terminal setting; compare every other flag and cc.
        restored[3] &= ~getattr(termios, 'PENDIN', 0)
        before = list(self.before)
        before[3] &= ~getattr(termios, 'PENDIN', 0)
        self.assertEqual(restored, before)
        self.assertEqual({p.name: p.read_bytes() for p in self.store.iterdir()}, self.saved)

    def test_arrows_skip_blocked_rows_and_launch_selected_upstream(self):
        self.start(['--allow-unverified'])
        output = self.read_until('Enter confirms')
        self.assertIn('mid/unknown-model [disabled: missing bundled model metadata]', output)
        self.assertIn('Target client: Codex CLI', output)
        os.write(self.master, b'\x1b[B\r')
        self.finish()
        self.assertEqual(json.loads(self.marker.read_text())['model'], KIMI)
        self.assertEqual(self.requests[-1][2]['model'], KIMI)
        self.assertEqual(self.requests[-1][1], 'Bearer synthetic-key')
        self.assertIn('ai_project_id=synthetic-project', self.requests[-1][0])

    def test_empty_catalog_does_not_open_picker(self):
        self.models = []
        self.start(['--allow-unverified'])
        self.read_until('no models available')
        self.finish(1)
        self.assertFalse(self.marker.exists())
        self.assertNotIn(b'Enter confirms', self.output)

    def test_launch_command_and_up_arrow(self):
        self.start(['launch', 'codex', '--allow-unverified', '--project-id', 'other-project'])
        self.read_until('Enter confirms')
        # Up wraps to the last eligible row; Down wraps back, skipping no choice.
        os.write(self.master, b'\x1bOA\x1bOB\r')
        self.finish()
        self.assertEqual(json.loads(self.marker.read_text())['model'], DEEPSEEK)
        self.assertTrue(all('ai_project_id=other-project' in r[0] for r in self.requests))

    def test_supported_bare_picker_ignores_saved_model(self):
        self.start([], self.supported)
        output = self.read_until('Enter confirms')
        self.assertIn(KIMI + ' [supported]', output)
        self.assertNotIn(DEEPSEEK.encode(), self.output)
        self.assertNotIn(GLM.encode(), self.output)
        os.write(self.master, b'\r')
        self.finish()
        self.read_until('Guardian: ' + GLM)
        self.assertEqual(json.loads(self.marker.read_text())['model'], KIMI)

    def test_supported_choices_follow_guardian_override(self):
        self.start(['launch', 'codex', '--guardian-model', KIMI], self.supported)
        output = self.read_until('Enter confirms')
        self.assertIn(DEEPSEEK + ' [supported]', output)
        self.assertNotIn(KIMI.encode(), self.output)
        os.write(self.master, b'\r')
        self.finish()
        self.read_until('Guardian: ' + KIMI)
        self.assertEqual(json.loads(self.marker.read_text())['model'], DEEPSEEK)

    def test_direct_picker_uses_route_support_and_native_reviewer(self):
        self.start(['launch', 'codex', '--direct'], self.supported)
        output = self.read_until('Enter confirms')
        self.assertIn(DEEPSEEK + ' [supported]', output)
        self.assertNotIn(KIMI.encode(), self.output)
        os.write(self.master, b'\r')
        self.finish()
        self.read_until('Guardian: native reviewer selection')
        self.assertEqual(json.loads(self.marker.read_text())['model'], DEEPSEEK)
        self.assertTrue(all(r[2] is None for r in self.requests))

    def test_cancel_restores_terminal_and_never_launches(self):
        # Each process owns its PTY; a cancellation must not start a client.
        for key in (b'\x1b', b'\x03', b'\x1b['):
            with self.subTest(key=key):
                self.start(['--allow-unverified'])
                self.read_until('Enter confirms')
                os.write(self.master, key)
                self.finish(1)
                self.assertFalse(self.marker.exists())

    def test_signal_restores_terminal(self):
        self.start(['--allow-unverified'])
        self.read_until('Enter confirms')
        self.process.send_signal(signal.SIGTERM)
        self.finish(1)
        self.assertFalse(self.marker.exists())

    def test_no_supported_models_explains_experimental_flag(self):
        self.start([])
        self.read_until('no supported main models')
        self.read_until('--allow-unverified')
        self.finish(1)
        self.assertFalse(self.marker.exists())
        self.assertNotIn(b'Enter confirms', self.output)

    def test_catalog_failure_never_launches(self):
        self.catalog_status = 503
        self.start(['--allow-unverified'])
        self.read_until('model catalog returned HTTP 503')
        self.finish(1)
        self.assertFalse(self.marker.exists())

    def test_all_blocked_rows_explain_reasons_without_prompt(self):
        self.models = ['outside/shortlist']
        self.start(['launch', 'codex', '--direct', '--allow-unverified'])
        self.read_until('outside/shortlist [disabled: missing bundled model metadata]')
        self.read_until('no eligible main models')
        self.finish(1)
        self.assertFalse(self.marker.exists())
        self.assertNotIn(b'Enter confirms', self.output)

    def test_explicit_main_bypasses_picker_and_saved_preference(self):
        self.start(['launch', 'codex', '--model', KIMI, '--allow-unverified'])
        self.finish()
        self.read_until('Main: ' + KIMI)
        self.assertNotIn(b'Enter confirms', self.output)
        self.assertEqual(json.loads(self.marker.read_text())['model'], KIMI)

    def test_invalid_explicit_id_never_prompts_or_launches(self):
        self.start(['launch', 'codex', '--model', 'absent/model', '--allow-unverified'])
        self.read_until('main model absent/model: not available')
        self.finish(1)
        self.assertFalse(self.marker.exists())
        self.assertNotIn(b'Enter confirms', self.output)

    def test_noninteractive_omitted_main_ignores_saved_preference(self):
        for args in ([], ['--allow-unverified'], ['launch', 'codex', '--allow-unverified']):
            with self.subTest(args=args):
                result = subprocess.run([str(self.binary), *args], input='', capture_output=True, text=True, env=self.env, timeout=5)
                self.assertEqual(result.returncode, 1)
                self.assertIn('--model ID', result.stderr)
                self.assertFalse(self.marker.exists())
        self.assertEqual(self.requests, [])

    def test_output_error_restores_terminal_without_launch(self):
        self.env['FIXTURE_OUTPUT_FAILURE'] = '1'
        self.start(['--allow-unverified'])
        self.read_until('synthetic terminal output failure')
        self.finish(1)
        self.assertFalse(self.marker.exists())

    def test_missing_guardian_does_not_prompt_or_substitute(self):
        self.models = [KIMI]
        self.start(['--allow-unverified'])
        self.read_until('Guardian model ' + GLM + ': not available')
        self.finish(1)
        self.assertFalse(self.marker.exists())
        self.assertNotIn(b'Enter confirms', self.output)


if __name__ == '__main__':
    unittest.main()
