"""Bounded release-acceptance sample, not model support promotion.

One two-turn coding session plus one controlled Guardian allow/deny pair.
Uses the existing capped observer and installed-client evaluation seams.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from types import SimpleNamespace

import live_compat as live
import model_evaluation as evaluation
from evaluation_candidates import candidate
from evaluation_report import approval_failures


def run(options, root):
    budget = root / 'budget.json'
    budget.write_text(json.dumps({'used': 0, 'maximum': 12}))
    names = ('TOFA_EVAL_BUDGET', 'TOFA_EVAL_CASE', 'TOFA_EVAL_MODEL', 'TOFA_EVAL_GUARDIAN_MODEL')
    before = {name: os.environ.get(name) for name in names}
    os.environ.update(TOFA_EVAL_BUDGET=str(budget), TOFA_EVAL_CASE='coding',
                      TOFA_EVAL_MODEL=options.model, TOFA_EVAL_GUARDIAN_MODEL=options.guardian_model)
    report = {'model': options.model, 'guardian_model': options.guardian_model,
              'support_status': 'Experimental', 'request_limit': 12, 'output_tokens_per_request': 4096,
              'coding_turn_seconds': 180, 'guardian_turn_seconds': 120, 'approvals': [], 'passed': False}
    try:
        coding_root = root / 'coding'; coding_root.mkdir()
        settings = SimpleNamespace(**vars(options))
        settings.timeout = 180
        settings.observer_harness = Path(evaluation.__file__).resolve()
        report['coding'] = live.run_one(settings, coding_root)
        # Stop this acceptance sample on failure; it is not the independent-lane campaign.
        if report['coding']['passed']:
            for case in ('allow', 'deny'):
                case_root = root / case; case_root.mkdir()
                result = evaluation.approval(settings, case_root, case)
                result['failures'] = approval_failures(result)
                result['passed'] = not result['failures']
                report['approvals'].append(result)
                if not result['passed']: break
        report['passed'] = (report['coding']['passed'] and len(report['approvals']) == 2
                            and all(case['passed'] for case in report['approvals']))
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError):
        report['failure'] = 'qualification_harness_failure'
    finally:
        report['used_requests'] = json.loads(budget.read_text())['used']
        for name, value in before.items():
            if value is None: os.environ.pop(name, None)
            else: os.environ[name] = value
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('launcher', 'codex', 'model', 'guardian-model', 'output'):
        parser.add_argument('--' + name, required=True)
    options = parser.parse_args()
    output = Path(options.output)
    if output.exists() or output.is_symlink() or not output.parent.is_dir():
        parser.error("output must be a new file in an existing directory")
    candidate(options.model); candidate(options.guardian_model)
    if os.name == 'nt':
        import windows_process
        options.codex = windows_process.resolve_codex(options.codex)
    with tempfile.TemporaryDirectory(prefix='tofa-acceptance-pair-') as directory:
        report = run(options, Path(directory).resolve())
    live.write_json(Path(options.output), report)
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
