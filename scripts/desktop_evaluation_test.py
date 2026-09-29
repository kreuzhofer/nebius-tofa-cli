"""Public evaluator through the real desktop launcher/engine; no paid inference."""
import http.server
import json
import os
from pathlib import Path
import shlex
import signal
import time
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

from evaluation_candidates import snapshot
from evaluation_proxy import fixture_response, message
from live_compat import LoopbackServer

ENGINE = os.environ.get('TOFA_TEST_DESKTOP_ENGINE')


@unittest.skipUnless(sys.platform == 'darwin' and ENGINE, 'set TOFA_TEST_DESKTOP_ENGINE on macOS')
class DesktopEvaluationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build = tempfile.TemporaryDirectory(prefix='tofa-eval-build-')
        cls.launcher = Path(cls.build.name) / 'launcher'
        subprocess.run(['go', 'build', '-o', str(cls.launcher), './scripts/fixtures/evaluation_launcher'], check=True)

    @classmethod
    def tearDownClass(cls):
        cls.build.cleanup()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='tofa-eval-test-', dir='/tmp')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.requests = []
        self.failure = None
        self.available = list(snapshot()['models'])
        self.review_started = threading.Event()
        self.review_cancelled = threading.Event()
        self.inspections = 0
        owner = self
        class Provider(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_): pass
            def do_GET(self):
                self.send_response(200); self.end_headers()
                self.wfile.write(json.dumps({'data': [{'id': m} for m in owner.available]}).encode())
            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                review = body.get('text', {}).get('format', {}).get('type') == 'json_schema'
                owner.requests.append(('guardian' if review else 'main', body['model']))
                if review:
                    owner.review_started.set()
                    if owner.failure == 'provider':
                        self.send_response(503); self.end_headers(); return
                    if owner.failure == 'cancel':
                        self.connection.settimeout(15)
                        try:
                            if not self.connection.recv(1): owner.review_cancelled.set()
                        except OSError:
                            pass
                        return
                    text = json.dumps({'outcome': 'deny' if 'prompt-injection fixture' in json.dumps(body) else 'allow'})
                    if owner.failure == 'review': text = '{"outcome":"maybe"}'
                    item = message(text)
                    if owner.failure == 'inspection':
                        inspected = any(x.get('type') == 'function_call_output' and
                            'tofa-review-inspection' in str(x.get('output')) and
                            'Process exited with code 0' in str(x.get('output')) for x in body.get('input', []))
                        if inspected:
                            owner.inspections += 1
                        else:
                            item = dict(id='fc_inspect', type='function_call', call_id='call_inspect_' + str(len(owner.requests)),
                                name='exec_command', status='completed', arguments=json.dumps({
                                    'cmd': 'printf tofa-review-inspection', 'max_output_tokens': 100}))
                else:
                    inputs = body.get('input', [])
                    last = max((i for i, x in enumerate(inputs) if x.get('role') == 'user'), default=-1)
                    if any(x.get('type') == 'function_call_output' for x in inputs[last + 1:]):
                        item = message('Checked the synthetic result. Finished.')
                    else:
                        continued = 'Extend summary.json' in json.dumps(inputs)
                        code = "import json; from pathlib import Path; n=json.loads(Path('input.json').read_text())['numbers']; s=dict(count=len(n),total=sum(n),max=max(n)); "
                        if continued: code += 's.update(min=min(n),average=sum(n)/len(n)); '
                        code += "Path('summary.json').write_text(json.dumps(s)); print('checked result')"
                        if owner.failure == 'environment': code += '; import os; assert os.environ.get("UNRELATED_API_KEY") is None'
                        if owner.failure == 'tool': code += '; raise SystemExit(7)'
                        item = dict(id='fc_eval', type='function_call', call_id='call_' + str(len(owner.requests)),
                                    name='exec_command', status='completed', arguments=json.dumps({
                                        'cmd': shlex.quote(sys.executable) + ' -c ' + shlex.quote(code),
                                        'max_output_tokens': 1000}))
                self.send_response(200); self.send_header('Content-Type', 'text/event-stream'); self.end_headers()
                if owner.failure == 'limit' and not review:
                    self.wfile.write(b'data: ' + b'x' * (256 * 1024 + 1) + b'\n\n'); return
                events = [json.loads(line[6:]) for line in fixture_response(item, body['model'],
                    {'input_tokens': 100, 'output_tokens': 20, 'total_tokens': 120}).decode().splitlines() if line]
                if owner.failure == 'usage': events[-1]['response'].pop('usage')
                if item['type'] == 'message' and not (review and owner.failure == 'inspection'):
                    text = item['content'][0]['text']
                    events[2:2] = [dict(type='response.output_text.delta', delta=delta,
                                       item_id=item['id'], output_index=0, content_index=0)
                                   for delta in (text[:len(text)//2], text[len(text)//2:])]
                for event in events:
                    self.wfile.write(('data: ' + json.dumps(event) + '\n\n').encode())
        self.server = LoopbackServer(('127.0.0.1', 0), Provider)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)
        self.env = dict(os.environ, FIXTURE_ENDPOINT='http://127.0.0.1:' + str(self.server.server_port),
                        HOME=str(self.root), CODEX_HOME=str(self.root / 'normal'), XDG_CONFIG_HOME=str(self.root / 'config'))
        (self.root / 'normal').mkdir()
        self.sentinel = self.root / 'normal/config.toml'
        self.sentinel.write_text('model="preserve-native"\n')
        self.metadata = self.root / 'metadata.json'
        self.metadata.write_text(json.dumps(snapshot()))

    def invoke(self, model='deepseek-ai/DeepSeek-V4.1-Flash', extra=(), live=False):
        output = self.root / ('report-' + str(len(list(self.root.glob('report-*')))) + '.json')
        result = subprocess.run([sys.executable, 'scripts/model_evaluation.py', '--desktop',
            '--launcher', str(self.launcher), '--codex', ENGINE, '--model', model,
            '--campaign', str(self.root / 'campaign'),
            '--output', str(output), *([] if live else ['--metadata-snapshot', str(self.metadata)]), *extra], env=self.env, capture_output=True, text=True, timeout=240)
        self.assertTrue(output.exists(), result.stderr)
        report = json.loads(output.read_text())
        evidence_dir = os.environ.get('TOFA_TEST_EVIDENCE_DIR')
        if evidence_dir:
            target = Path(evidence_dir); target.mkdir(parents=True, exist_ok=True)
            (target / (self._testMethodName + '-' + report['run_id'] + '.json')).write_text(json.dumps(report, indent=2) + '\n')
        return result, report

    def test_command_to_report_checks_tools_resume_approvals_and_preservation(self):
        result, report = self.invoke()
        self.assertEqual(result.returncode, 0, json.dumps(report) + result.stderr)
        self.assertEqual(report['status'], 'passed')
        self.assertEqual([c['status'] for c in report['cases']], ['passed'] * 9)
        self.assertTrue(all(c['same_session'] for c in report['cases'][:3]))
        approvals = report['cases'][3:]
        self.assertEqual([c['command_executed'] for c in approvals], [True, False] * 3)
        self.assertEqual(len(report['requests']), len(self.requests) + 12)
        self.assertEqual(len({r['request_id'] for r in report['requests']}), len(report['requests']))
        self.assertTrue(all(m == ('zai-org/GLM-5.3-Flash' if role == 'guardian' else report['model']) for role, m in self.requests))
        self.assertEqual(self.sentinel.read_text(), 'model="preserve-native"\n')
        self.assertTrue(report['normal_settings_preserved'])
        self.assertNotIn('synthetic-fixture-token', json.dumps(report))
        self.assertNotIn('checked result', json.dumps(report))
        self.assertEqual(report['naming']['generated_title'], 'unmeasured')

    def test_remaining_exact_candidates_use_the_same_baseline(self):
        for model in ('zai-org/GLM-5.3-Flash', 'zai-org/GLM-5.3', 'moonshotai/Kimi-K3', 'nvidia/Nemotron-3-Ultra-550b-a55b'):
            with self.subTest(model=model):
                self.requests.clear()
                result, report = self.invoke(model)
                self.assertEqual(result.returncode, 0, json.dumps(report))
                self.assertEqual([c['status'] for c in report['cases']], ['passed'] * 9)
                self.assertTrue(all(c['same_session'] for c in report['cases'][:3]))
                self.assertEqual(report['guardian_model'], 'zai-org/GLM-5.3-Flash')
                self.assertEqual(len(report['requests']), len(self.requests) + 12)

    def test_public_command_refreshes_metadata_and_prices_before_inference(self):
        from model_evaluation import main
        model = 'zai-org/GLM-5.3-Flash'
        result, prerequisite = self.invoke(model)
        self.assertEqual(result.returncode, 0)
        qualification = self.root / 'qualification.json'
        qualification.write_text(json.dumps(prerequisite))
        public = json.dumps([{'flavors': [{'model_id': model, 'max_model_len': 1024000,
            'use_cases': ['text', 'image', 'responses_api', 'function_calling'],
            'input_price_per_million_tokens': 0.9, 'output_price_per_million_tokens': 2.0}]}]).encode()
        output = self.root / 'refreshed.json'
        args = ['model_evaluation.py', '--desktop', '--launcher', str(self.launcher), '--codex', ENGINE,
                '--model', model, '--campaign', str(self.root / 'campaign'),
                '--qualification-evidence', str(qualification), '--output', str(output)]
        previous_signal = signal.getsignal(signal.SIGTERM)
        try:
            with patch.dict(os.environ, self.env, clear=True), patch('sys.argv', args), patch(
                    'desktop_evaluation.fetch_public', return_value=public):
                self.assertEqual(main(), 0)
        finally:
            signal.signal(signal.SIGTERM, previous_signal)
        refreshed = json.loads(output.read_text())
        self.assertEqual(refreshed['rates'][model]['input_usd_per_million'], 0.9)
        self.assertEqual(refreshed['rates'][model]['output_usd_per_million'], 2.0)
        self.assertEqual(refreshed['cost']['main']['estimated_usd'], 0.00156)
        self.assertEqual(refreshed['cost']['guardian']['estimated_usd'], 0.00078)
        matrix_output = self.root / 'conditions.json'
        subprocess.run([sys.executable, 'scripts/model_evaluation.py', '--desktop-report',
                        str(self.root / 'campaign'), '--output', str(matrix_output)], check=True)
        candidate = next(c for c in json.loads(matrix_output.read_text())['candidates'] if c['model'] == model)
        self.assertEqual(candidate['status'], 'multiple_conditions')
        self.assertEqual(len(candidate['conditions']), 2)
        self.assertEqual({c['evidence_kind'] for c in candidate['conditions']}, {'live-provider', 'controlled-provider'})
        count = len(self.requests)
        for index, malformed in enumerate(([None], [{'flavors': [None]}])):
            args[-1] = str(self.root / ('invalid-' + str(index) + '.json'))
            try:
                with patch.dict(os.environ, self.env, clear=True), patch('sys.argv', args), patch(
                        'desktop_evaluation.fetch_public', return_value=json.dumps(malformed).encode()):
                    self.assertEqual(main(), 1)
            finally:
                signal.signal(signal.SIGTERM, previous_signal)
            blocked = json.loads(Path(args[-1]).read_text())
            self.assertEqual(blocked['status'], 'blocked')
            self.assertEqual(blocked['reason'], 'candidate_metadata_unresolved')
        self.assertEqual(len(self.requests), count)

    def test_child_environment_excludes_unrelated_credentials(self):
        self.failure = 'environment'
        self.env['UNRELATED_API_KEY'] = 'private-parent-sentinel'
        result, report = self.invoke()
        self.assertEqual(result.returncode, 0, json.dumps(report))
        self.assertNotIn('private-parent-sentinel', json.dumps(report))

    def test_saved_keychain_login_is_readable_with_isolated_desktop_home(self):
        # Simulate the macOS vault's HOME-relative lookup at the executable
        # boundary, then run the real launcher and engine against the provider.
        keychains = self.root / 'Library/Keychains'
        keychains.mkdir(parents=True)
        sentinel = keychains / 'synthetic-login'
        sentinel.write_text('synthetic-vault-sentinel')
        launcher = self.root / 'vault-launcher'
        launcher.write_text('#!' + sys.executable + '\n' +
            'import os, sys\nfrom pathlib import Path\n' +
            'if sys.argv[1:2] == ["launch"]:\n' +
            '    assert Path.home() != Path(' + repr(str(self.root)) + ')\n' +
            '    assert (Path.home() / "Library/Keychains/synthetic-login").read_text() == "synthetic-vault-sentinel"\n' +
            'os.execv(' + repr(str(self.launcher)) + ', [' + repr(str(self.launcher)) + ', *sys.argv[1:]])\n')
        launcher.chmod(0o700)
        self.launcher = launcher
        result, report = self.invoke()
        self.assertEqual(result.returncode, 0, json.dumps(report))
        self.assertEqual([c['status'] for c in report['cases']], ['passed'] * 9)
        self.assertEqual(sentinel.read_text(), 'synthetic-vault-sentinel')
        self.assertTrue(report['normal_settings_preserved'])
        self.assertNotIn('synthetic-vault-sentinel', json.dumps(report))

    def test_live_run_without_matching_qualification_is_blocked_before_inference(self):
        result, report = self.invoke(live=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report['status'], 'blocked')
        self.assertEqual(report['reason'], 'qualification_prerequisite_failed')
        self.assertEqual(self.requests, [])


    def test_failed_tool_result_stops_affected_sessions_but_retains_approvals(self):
        self.failure = 'tool'
        result, report = self.invoke()
        self.assertEqual(result.returncode, 1)
        self.assertEqual([c['status'] for c in report['cases'][:3]], ['failed', 'unattempted', 'unattempted'])
        self.assertEqual(report['cases'][0]['turns'][0]['tools_failed'], 1)
        self.assertTrue(report['cases'][0]['turns'][0]['tool_result_continued'])
        self.assertEqual([c['status'] for c in report['cases'][3:]], ['passed'] * 6)

    def test_guardian_inspection_continues_before_the_final_assessment(self):
        self.failure = 'inspection'
        result, report = self.invoke()
        self.assertEqual(result.returncode, 0, json.dumps(report))
        self.assertEqual(self.inspections, 6)
        self.assertEqual([c['decisions'] for c in report['cases'][3:]], [['allow'], ['deny']] * 3)
        self.assertEqual([c['command_executed'] for c in report['cases'][3:]], [True, False] * 3)
        reviews = [r for r in report['requests'] if r.get('role') == 'guardian']
        self.assertEqual(len(reviews), 12)
        self.assertEqual(sum(r.get('review_tool_calls', 0) for r in reviews), 6)
        self.assertEqual(report['cost']['guardian']['paid_requests'], 12)

    def test_invalid_review_counts_native_attempts_and_observes_nonexecution(self):
        self.failure = 'review'
        result, report = self.invoke()
        self.assertEqual(result.returncode, 1)
        self.assertEqual([c['status'] for c in report['cases'][:3]], ['passed'] * 3)
        approval = report['cases'][3]
        self.assertEqual(approval['decisions'], ['invalid'] * 3)
        self.assertFalse(approval['command_executed'])
        self.assertIn('assessment_invalid', approval['failures'])
        self.assertEqual(len([r for r in report['requests'] if r['role'] == 'guardian']), 3)
        self.assertEqual([c['status'] for c in report['cases'][4:]], ['unattempted'] * 5)

    def test_provider_errors_retain_each_native_attempt(self):
        self.failure = 'provider'
        result, report = self.invoke()
        self.assertEqual(result.returncode, 1)
        self.assertIn('upstream_error', report['cases'][3]['failures'])
        self.assertFalse(report['cases'][3]['command_executed'])
        self.assertEqual(len([r for r in report['requests'] if r['role'] == 'guardian']), 4)
        self.assertIsNone(report['cost']['guardian']['estimated_usd'])

    def test_missing_rates_and_usage_never_block_qualification(self):
        self.failure = 'usage'
        metadata = json.loads(self.metadata.read_text())
        for model in metadata['models'].values(): model.pop('input_price')
        self.metadata.write_text(json.dumps(metadata))
        result, report = self.invoke()
        self.assertEqual(result.returncode, 0, json.dumps(report))
        self.assertIsNone(report['cost']['main']['estimated_usd'])
        self.assertIsNone(report['cost']['guardian']['estimated_usd'])

    def test_local_stream_limit_is_retained_without_stopping_independent_lane(self):
        self.failure = 'limit'
        result, report = self.invoke()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report['cases'][0]['status'], 'failed')
        self.assertIn('event_body_limit', [r.get('failure') for r in report['requests']])
        self.assertEqual([c['status'] for c in report['cases'][3:]], ['passed'] * 6)

    def test_preflight_blocked_candidates_do_not_reach_inference(self):
        self.available.remove('deepseek-ai/DeepSeek-V4.1-Flash')
        result, report = self.invoke()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report['reason'], 'candidate_unavailable')
        self.assertTrue(all(c['status'] == 'blocked' for c in report['cases']))
        self.assertEqual(self.requests, [])
        self.assertIsNone(report['cost']['main']['estimated_usd'])

    def test_restart_report_does_not_replay_or_double_count_attempts(self):
        result, original = self.invoke()
        self.assertEqual(result.returncode, 0)
        count = len(self.requests)
        output = self.root / 'recovered.json'
        subprocess.run([sys.executable, 'scripts/model_evaluation.py', '--desktop-report',
                        str(self.root / 'campaign' / original['run_id']), '--output', str(output)], check=True)
        self.assertEqual(json.loads(output.read_text()), original)
        self.assertEqual(len(self.requests), count)
        result, diagnostic = self.invoke(extra=['--diagnostic', 'repeat-baseline'])
        self.assertEqual(result.returncode, 0)
        self.assertNotEqual(diagnostic['run_id'], original['run_id'])
        self.assertEqual(diagnostic['diagnostic'], 'repeat-baseline')
        self.assertTrue((self.root / 'campaign' / original['run_id'] / 'run.json').exists())

    def test_changed_metadata_blocks_and_preserves_five_candidate_matrix(self):
        metadata = json.loads(self.metadata.read_text())
        metadata['models']['deepseek-ai/DeepSeek-V4.1-Flash']['context_window'] = 12345
        self.metadata.write_text(json.dumps(metadata))
        result, report = self.invoke()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report['reason'], 'candidate_metadata_unresolved')
        self.assertEqual(self.requests, [])
        output = self.root / 'matrix.json'
        subprocess.run([sys.executable, 'scripts/model_evaluation.py', '--desktop-report',
                        str(self.root / 'campaign'), '--output', str(output)], check=True)
        matrix = json.loads(output.read_text())
        self.assertEqual(len(matrix['candidates']), 5)
        self.assertEqual(matrix['candidates'][0]['status'], 'blocked')
        self.assertEqual([c['status'] for c in matrix['candidates'][1:]], ['unattempted'] * 4)
        self.assertEqual(matrix['native_attempts'], 0)


    def test_native_cancellation_retains_incomplete_review_and_no_execution(self):
        self.failure = 'cancel'
        result, report = self.invoke(extra=['--diagnostic', 'review-cancellation', '--cancel-review'])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report['cases'][3]['status'], 'incomplete')
        self.assertFalse(report['cases'][3]['command_executed'])
        self.assertTrue(self.review_cancelled.wait(3), 'native cancellation did not reach the provider')
        self.assertTrue(any(r.get('failure') == 'cancelled' for r in report['requests']))



class CodingDeadlineTests(unittest.TestCase):
    def test_extended_deadline_reaches_observer_turn_and_process_wait(self):
        from types import SimpleNamespace
        import desktop_evaluation as evaluation
        from unittest.mock import MagicMock
        manifest, fingerprint = evaluation.effective_manifest(360)
        self.assertEqual(manifest['limits']['coding_request_seconds'], 360)
        self.assertEqual(manifest['limits']['coding_turn_seconds'], 360)
        self.assertEqual(manifest['limits']['approval_turn_seconds'], 120)
        self.assertEqual(manifest['limits']['native_review_seconds'], 90)
        baseline, baseline_fingerprint = evaluation.effective_manifest(180)
        self.assertNotEqual(fingerprint, baseline_fingerprint)
        self.assertEqual(baseline, evaluation.read(evaluation.MANIFEST))
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'workspace').mkdir()
            app, config = evaluation.bundle(root, '/engine', 'zai-org/GLM-5.3-Flash', manifest['limits'])
            state = {'case': 'coding', 'observations': str(root / 'requests.json'), 'result': str(root / 'result.json')}
            evaluation.save(config['state'], state)
            process = MagicMock()
            process.wait.return_value = 0
            options = SimpleNamespace(launcher='/launcher', model=config['model'], evidence={'manifest': manifest})
            with patch('desktop_evaluation.subprocess.Popen', return_value=process):
                evaluation.launch(options, root, app, state)
            process.wait.assert_called_once_with(timeout=420)
            with patch.dict(os.environ, TOFA_API_KEY='synthetic', TOFA_DESKTOP_CONTEXT='synthetic'), patch(
                    'desktop_evaluation.EvaluationProxy') as proxy, patch('desktop_evaluation.subprocess.call'):
                proxy.return_value.__enter__.return_value.url = 'http://127.0.0.1:2'
                evaluation.observe(config, ['model_providers.nebius-tofa.base_url="http://127.0.0.1:1"'])
            self.assertEqual(proxy.call_args.args[5], 360)
            engine = MagicMock()
            engine.call.side_effect = [{'thread': {'id': 'thread'}, 'model': config['model'], 'modelProvider': 'nebius-tofa'}, {'turn': {'id': 'turn'}}]
            engine.receive.return_value = {'method': 'turn/completed', 'params': {'threadId': 'thread', 'turn': {'status': 'completed'}}}
            profile = root / 'profile'; profile.mkdir()
            with patch.dict(os.environ, CODEX_ELECTRON_USER_DATA_PATH=str(profile), CODEX_CLI_PATH='/bridge'), patch(
                    'desktop_evaluation.Engine', return_value=engine), patch('desktop_evaluation.time.monotonic', return_value=100):
                evaluation.desktop(config)
            self.assertEqual(engine.call.call_args_list[1].args[2], 460)
            engine.receive.assert_called_once_with(460)

    def test_extended_observer_accepts_360_but_rejects_larger_deadlines(self):
        from evaluation_proxy import EvaluationProxy
        with tempfile.TemporaryDirectory() as temporary:
            with EvaluationProxy('http://127.0.0.1:1', 'synthetic', Path(temporary) / 'requests.json', None, 'coding', 360):
                pass
            with self.assertRaises(ValueError):
                EvaluationProxy('http://127.0.0.1:1', 'synthetic', Path(temporary) / 'requests.json', None, 'coding', 361)


class CaseFailureTests(unittest.TestCase):
    def test_failed_relaunch_without_turn_result_is_recorded_without_raising(self):
        from types import SimpleNamespace
        import desktop_evaluation as evaluation
        entry = {'id': 'coding-1', 'kind': 'coding', 'status': 'unattempted'}
        options = SimpleNamespace(codex='/engine', model='moonshotai/Kimi-K3',
            evidence={'manifest': evaluation.read(evaluation.MANIFEST), 'cases': [entry]})
        def failed_relaunch(options, root, app, state):
            if state['resume']:
                return {'turn_completed': False, 'client_error': True, 'exit_code': 1}
            (root / 'workspace/summary.json').write_text('{"count":4,"total":18,"max":9}')
            evaluation.save(state['observations'], [{'completed': True, 'status': 200,
                'text_deltas': 2, 'tool_results': 1}])
            return {'turn_completed': True, 'client_error': False, 'tools_succeeded': 1,
                    'tools_failed': 0, 'same_session': False}
        with tempfile.TemporaryDirectory() as temporary, patch('desktop_evaluation.bundle',
                return_value=('/app', {})), patch('desktop_evaluation.launch', side_effect=failed_relaunch):
            evaluation.case(options, Path(temporary), entry)
        self.assertEqual(entry['status'], 'failed')
        self.assertTrue(entry['turns'][0]['passed'])
        self.assertFalse(entry['turns'][1]['passed'])
        self.assertFalse(entry['same_session'])
        self.assertTrue(entry['scratch_credentials_preserved'])


class EvaluationCleanupReportTests(unittest.TestCase):
    def test_cleanup_failure_keeps_completed_and_unattempted_cases_distinct(self):
        from model_evaluation import main
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            launcher, engine = root / 'launcher', root / 'engine'
            launcher.write_text('#!/bin/sh\nif [ "$1" = --version ]; then echo "tofa evaluation-fixture"; else echo "deepseek-ai/DeepSeek-V4.1-Flash"; echo "zai-org/GLM-5.3-Flash"; fi\n')
            engine.write_text('#!/bin/sh\necho "codex-cli 0.155.0-alpha.16.4"\n')
            launcher.chmod(0o700); engine.chmod(0o700)
            metadata = root / 'metadata.json'; metadata.write_text(json.dumps(snapshot()))
            output = root / 'report.json'
            args = ['model_evaluation.py', '--desktop', '--launcher', str(launcher),
                    '--codex', str(engine), '--model', 'deepseek-ai/DeepSeek-V4.1-Flash',
                    '--campaign', str(root / 'campaign'), '--metadata-snapshot', str(metadata),
                    '--output', str(output)]
            def case(options, run, entry):
                if entry['id'] == 'coding-1':
                    entry['status'] = 'passed'
                else:
                    entry['status'] = 'incomplete'
                    raise ValueError('evaluation_bridge_cleanup_failed')
            previous_signal = signal.getsignal(signal.SIGTERM)
            try:
                with patch('sys.argv', args), patch.dict(os.environ, HOME=str(root),
                        CODEX_HOME=str(root / 'native'), XDG_CONFIG_HOME=str(root / 'config')), patch(
                        'desktop_evaluation.platform.system', return_value='Darwin'), patch(
                        'desktop_evaluation.platform.machine', return_value='arm64'), patch(
                        'desktop_evaluation.platform.mac_ver', return_value=('26.6.2', (), '')), patch(
                        'desktop_evaluation.case', side_effect=case):
                    self.assertEqual(main(), 1)
            finally:
                signal.signal(signal.SIGTERM, previous_signal)
            report = json.loads(output.read_text())
            self.assertEqual(report['status'], 'incomplete')
            self.assertEqual(report['reason'], 'evaluation_bridge_cleanup_failed')
            self.assertEqual([c['status'] for c in report['cases']],
                             ['passed', 'incomplete'] + ['unattempted'] * 7)


class FileDiagnosticsTests(unittest.TestCase):
    def test_distinguishes_wrong_output_from_changed_input_without_raw_content(self):
        from desktop_evaluation import file_diagnostics
        from live_compat import digest
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'workspace').mkdir()
            source = root / 'workspace/input.json'
            output = root / 'workspace/summary.json'
            source.write_text('{"numbers":[4,-2,7,9]}')
            original = digest(source)
            output.write_text('{"count":4,"total":17,"max":9,"secret":"do-not-retain"}')
            result = file_diagnostics(root, 0, original)
            self.assertTrue(result['input_unchanged'])
            self.assertFalse(result['summary_correct'])
            self.assertEqual(result['numeric_fields']['total'], {'expected':18, 'actual':17})
            self.assertEqual(result['unexpected_field_count'], 1)
            self.assertNotIn('secret', json.dumps(result))
            self.assertNotIn('do-not-retain', json.dumps(result))
            output.write_text(json.dumps({'count': 4, 'total': 10**400, 'max': 9}))
            huge = file_diagnostics(root, 0, original)
            self.assertFalse(huge['summary_correct'])
            self.assertEqual(huge['numeric_fields']['total']['actual'], 10**400)
            source.write_text('changed')
            output.write_text('{"count":4,"total":18,"max":9}')
            result = file_diagnostics(root, 0, original)
            self.assertFalse(result['input_unchanged'])
            self.assertTrue(result['summary_correct'])

    def test_missing_malformed_and_nonnumeric_outputs_are_explicit(self):
        from desktop_evaluation import file_diagnostics
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / 'workspace').mkdir()
            output = root / 'workspace/summary.json'
            self.assertEqual(file_diagnostics(root, 0, 'original')['summary_state'], 'missing')
            output.write_text('not-json')
            self.assertEqual(file_diagnostics(root, 0, 'original')['summary_state'], 'invalid_json')
            output.write_text('{"count":true,"total":"do-not-retain","max":NaN}')
            result = file_diagnostics(root, 0, 'original')
            self.assertFalse(result['summary_correct'])
            self.assertTrue(all(v['actual'] is None for v in result['numeric_fields'].values()))
            self.assertNotIn('do-not-retain', json.dumps(result))


class EvaluationBridgeCleanupTests(unittest.TestCase):
    def test_launch_retires_only_its_verified_bridge_on_success_and_failure(self):
        from desktop_evaluation import launch
        from types import SimpleNamespace
        import hashlib
        for code, conflict in ((0, None), (1, None), (0, "edited"), (0, "preexisting")):
            with self.subTest(exit_code=code, conflict=conflict), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                (root / 'workspace').mkdir()
                config = root / 'ordinary-config'
                other = config / 'tofa/desktop-bridge-v2/ordinary-bridge'
                other.mkdir(parents=True)
                (other / 'sentinel').write_text('ordinary saved reference')
                app = root / 'Evaluation.app'
                engine = str(app / 'Contents/Resources/codex')
                bridge = other.parent / hashlib.sha256(engine.encode()).hexdigest()
                launcher = root / 'launcher'
                launcher.write_text('#!' + sys.executable + '\n' +
                    'import hashlib,json,os,sys\nfrom pathlib import Path\n' +
                    'engine=' + repr(engine) + '\n' +
                    'p=Path(os.environ["XDG_CONFIG_HOME"])/"tofa/desktop-bridge-v2"/hashlib.sha256(engine.encode()).hexdigest()\n' +
                    'p.mkdir()\nbinary=b"owned evaluation helper"\n' +
                    '(p/"tofa-desktop-engine").write_bytes(binary)\n' +
                    '(p/"owner.json").write_text(json.dumps(dict(Version=2,Engine=engine,SHA256=hashlib.sha256(binary).hexdigest())))\n' +
                    ('(p/"tofa-desktop-engine").write_bytes(b"edited helper")\n' if conflict == 'edited' else '') +
                    'sys.exit(' + str(code) + ')\n')
                launcher.chmod(0o700)
                options = SimpleNamespace(launcher=str(launcher), model='example',
                    evidence={'manifest': {'limits': {'coding_turn_seconds': 1}}})
                state = {'case': 'coding', 'result': str(root / 'result.json')}
                if conflict == 'preexisting':
                    bridge.mkdir()
                    (bridge / 'sentinel').write_text('preexisting saved reference')
                with patch.dict(os.environ, {'XDG_CONFIG_HOME': str(config)}):
                    if conflict:
                        reason = 'evaluation_bridge_preexisting' if conflict == 'preexisting' else 'evaluation_bridge_cleanup_failed'
                        with self.assertRaisesRegex(ValueError, reason):
                            launch(options, root, app, state)
                        self.assertTrue(bridge.exists())
                        self.assertEqual((other / 'sentinel').read_text(), 'ordinary saved reference')
                        continue
                    # Repeated turns must recreate the same disposable reference.
                    for _ in range(2):
                        launch(options, root, app, state)
                        self.assertFalse(bridge.exists(), 'evaluation bridge blocks future installed upgrades')
                self.assertEqual((other / 'sentinel').read_text(), 'ordinary saved reference')


if __name__ == '__main__': unittest.main()
