"""Public CLI tests for evaluation reports; all evidence is synthetic."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('model_evaluation.py')


class EvaluationTests(unittest.TestCase):
    def score(self, turn, **overrides):
        evidence = {'model': 'moonshotai/Kimi-K3', 'codex_version': 'codex-cli 0.155.1',
                    'platform': 'Darwin/arm64', 'route': 'adapted with test-only loopback SSE observer',
                    'started_utc': '2026-09-22T00:00:00Z', 'normal_settings_preserved': True,
                    'runs': [{'turns': [turn, turn], 'same_session': True,
                              'scratch_settings_preserved': True}], **overrides}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, output = root / 'source.json', root / 'report.json'
            source.write_text(json.dumps(evidence))
            result = subprocess.run([sys.executable, str(SCRIPT), '--score', str(source),
                                     '--output', str(output)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(output.read_text()), output.read_text()

    def test_quality_failure_does_not_hide_protocol_pass(self):
        turn = {'exit_code': 0, 'turn_completed': True, 'tools_succeeded': 1,
                'files_correct': False, 'streaming_observed': True,
                'elapsed_ms': 1240, 'streams': [{'status': 200, 'completed': True,
                    'headers_ms': 120, 'first_delta_ms': 220, 'completed_ms': 1200}],
                'private': 'PRIVATE_SESSION'}
        report, raw = self.score(turn)
        self.assertEqual(report['runs'][0]['protocol']['score'], 6)
        self.assertEqual(report['runs'][0]['coding']['score'], 0)
        self.assertEqual(report['runs'][0]['failures'], ['coding_incorrect'])
        self.assertEqual(report['automatic_approval']['status'], 'not_measured')
        self.assertEqual(report['status'], 'incomplete')
        self.assertNotIn('PRIVATE_SESSION', raw)

    def test_deadline_is_incomplete_not_a_coding_mistake(self):
        report, _ = self.score({'exit_code': -15, 'turn_completed': False,
            'tools_succeeded': 0, 'files_correct': False, 'timed_out': True,
            'streams': [{'status': 0, 'completed': False, 'stage': 'response_headers'}]})
        self.assertEqual(report['runs'][0]['failures'], ['deadline_incomplete'])
        self.assertEqual(report['runs'][0]['coding']['measured'], 0)
        self.assertEqual(report['runs'][0]['latency_classification'], 'adapter_or_provider_wait_incomplete')

    def test_local_budget_stop_is_distinct_from_provider_rejection(self):
        report, _ = self.score({'exit_code': 1, 'turn_completed': False,
            'streams': [{'status': 429, 'completed': False,
                         'failure': 'request_budget_limit'}]})
        self.assertEqual(report['runs'][0]['failures'], ['request_budget_limit'])
        self.assertEqual(report['runs'][0]['coding']['measured'], 0)

    def test_proxy_timeout_retains_its_cause(self):
        report, _ = self.score({'exit_code': 1, 'turn_completed': False,
            'streams': [{'status': 200, 'completed': False,
                         'failure': 'deadline_incomplete', 'stage': 'stream_read'}]})
        self.assertEqual(report['runs'][0]['failures'], ['deadline_incomplete'])
        self.assertEqual(report['runs'][0]['latency_classification'], 'adapter_or_provider_wait_incomplete')

    def test_report_keeps_safe_file_diagnostics_and_preservation(self):
        report, raw = self.score({'exit_code': 0, 'turn_completed': True,
            'files_correct': False, 'file_checks': {'summary_status': 'object',
                'unexpected_field_count': 1, 'expected_fields_match': {'count': True,
                    'total': False, 'max': True, 'PRIVATE_FIELD': 'PRIVATE_VALUE'},
                'input_preserved': True, 'raw_content': 'PRIVATE_CONTENT'}},
            normal_settings_preserved=False)
        self.assertFalse(report['normal_settings_preserved'])
        checks = report['runs'][0]['file_checks'][0]
        self.assertEqual(checks['expected_fields_match'], {'count': True, 'total': False, 'max': True})
        self.assertEqual(checks['unexpected_field_count'], 1)
        self.assertTrue(checks['input_preserved'])
        self.assertTrue(report['runs'][0]['scratch_settings_preserved'])
        self.assertNotIn('PRIVATE', raw)

    def test_invalid_measurement_fails_without_exporting_private_content(self):
        evidence = {'model': 'moonshotai/Kimi-K3', 'runs': [{'turns': [
            {'elapsed_ms': 'PRIVATE_VALUE', 'streams': []}]}]}
        with tempfile.TemporaryDirectory() as directory:
            source, output = Path(directory) / 'source.json', Path(directory) / 'report.json'
            source.write_text(json.dumps(evidence))
            result = subprocess.run([sys.executable, str(SCRIPT), '--score', str(source),
                                     '--output', str(output)], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists())
            self.assertNotIn('PRIVATE_VALUE', result.stdout + result.stderr)

    def test_extra_turns_cannot_inflate_the_protocol_score(self):
        evidence = {'runs': [{'turns': [{}, {}, {}]}]}
        with tempfile.TemporaryDirectory() as directory:
            source, output = Path(directory) / 'source.json', Path(directory) / 'report.json'
            source.write_text(json.dumps(evidence))
            result = subprocess.run([sys.executable, str(SCRIPT), '--score', str(source),
                                     '--output', str(output)], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists())

    def test_usage_and_request_stage_remain_available_without_raw_fields(self):
        report, raw = self.score({'exit_code': 0, 'turn_completed': True,
            'streams': [{'status': 200, 'completed': True, 'stage': 'stream_read',
                         'usage': {'input_tokens': 123, 'output_tokens': 17, 'private': 'PRIVATE'}}]})
        request = report['runs'][0]['timing'][0]['requests'][0]
        self.assertEqual(request['usage'], {'input_tokens': 123, 'output_tokens': 17})
        self.assertEqual(request['stage'], 'stream_read')
        self.assertNotIn('PRIVATE', raw)

    def test_completed_stream_with_prior_error_cannot_pass_protocol(self):
        report, _ = self.score({'exit_code': 0, 'turn_completed': True, 'tools_succeeded': 1,
            'files_correct': True, 'streaming_observed': True,
            'streams': [{'status': 200, 'completed': True, 'failure': 'response_incomplete'}]})
        self.assertEqual(report['runs'][0]['protocol']['score'], 4)
        self.assertEqual(report['runs'][0]['coding']['score'], 2)
        self.assertIn('response_incomplete', report['runs'][0]['failures'])


if __name__ == '__main__':
    unittest.main()
