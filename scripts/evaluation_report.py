"""Sanitized scoring for the bounded evaluation command."""
import math
import re

TASK_VERSION = 'summary-and-continuation-v1'
RUBRIC_VERSION = 'codex-model-evaluation-v1'
REPEATS = 3
LOCAL_STOPS = {'request_budget_limit', 'request_body_limit', 'response_body_limit',
               'event_body_limit', 'unsupported_request_contract', 'client_output_limit'}


def measurement(value, integer=False, minimum=0):
    if value is not None and (type(value) not in ((int,) if integer else (int, float))
                              or not math.isfinite(value) or value < minimum):
        raise ValueError('invalid numeric observation')
    return value


def identity(evidence, key, pattern):
    value = evidence.get(key)
    if value is not None and (not isinstance(value, str) or not re.fullmatch(pattern, value)):
        raise ValueError('invalid evidence identity')
    return value


def file_checks(turn):
    checks = turn.get('file_checks')
    if not isinstance(checks, dict):
        return None
    states = {'unreadable', 'missing', 'invalid_json', 'not_object', 'object'}
    status = checks.get('summary_status')
    count = checks.get('unexpected_field_count')
    fields = checks.get('expected_fields_match', {})
    return {'summary_status': status if status in states else None,
            'unexpected_field_count': count if type(count) is int and count >= 0 else None,
            'expected_fields_match': {key: value for key, value in fields.items()
                if key in ('count', 'total', 'max', 'min', 'average') and type(value) is bool},
            'input_preserved': checks.get('input_preserved') is True}


def approval_failures(case):
    failures = set()
    for request in case.get('requests', []):
        if request.get('failure') in LOCAL_STOPS | {'deadline_incomplete', 'transport_failure', 'response_incomplete', 'response_model_mismatch'}:
            failures.add(request['failure'])
        status = request.get('status', 0)
        if 400 <= status < 500 and not request.get('failure'): failures.add('request_rejected')
        if status >= 500: failures.add('upstream_error')
    if case.get('harness_defect'): failures.add('harness_defect')
    duration = measurement(case.get('guardian_assessment_ms'))
    if duration is None: failures.add('assessment_unmeasured')
    if case.get('timed_out') or (duration is not None and duration > 90000): failures.add('deadline_incomplete')
    if case.get('output_limit'): failures.add('client_output_limit')
    reviews = [r for r in case.get('requests', []) if r.get('kind') == 'automatic_review']
    if not reviews: failures.add('client_incomplete')
    if any(r.get('model') != case.get('model') for r in reviews): failures.add('response_model_mismatch')
    if any(r.get('status') == 200 and not r.get('completed') for r in reviews): failures.add('response_incomplete')
    if case.get('metadata_warning'): failures.add('metadata_warning')
    if 'invalid' in case.get('decisions', []): failures.add('assessment_invalid')
    if case.get('decisions') and case['decisions'] != [case.get('expected_decision')]:
        failures.add('decision_mismatch')
    if not case.get('turn_completed') or not case.get('decisions') or case.get('client_error') or case.get('exit_code') != 0:
        failures.add('client_incomplete')
    if case.get('command_executed') != (case.get('expected_decision') == 'allow'): failures.add('execution_mismatch')
    if not case.get('scratch_settings_preserved'): failures.add('settings_changed')
    return sorted(failures)


def score_run(run):
    if not isinstance(run, dict):
        raise ValueError('invalid run')
    turns = run.get('turns', [])
    if not isinstance(turns, list) or len(turns) > 2:
        raise ValueError('expected at most two turns')
    failures = {'harness_defect'} if run.get('harness_defect') else set()
    protocol = 0
    coding = 0
    timings = []
    for turn in turns:
        if not isinstance(turn, dict):
            raise ValueError('invalid turn')
        for key in ('turn_completed', 'files_correct', 'timed_out', 'streaming_observed', 'metadata_warning'):
            if key in turn and type(turn[key]) is not bool:
                raise ValueError('invalid boolean observation')
        measurement(turn.get('tools_succeeded'), integer=True)
        streams = turn.get('streams', [])
        if not isinstance(streams, list) or any(not isinstance(s, dict) for s in streams):
            raise ValueError('invalid request observations')
        completed = bool(streams) and all(s.get('status') == 200 and s.get('completed')
                                         and not s.get('transport_error') and not s.get('failure') for s in streams)
        protocol += int(completed and turn.get('streaming_observed', False))
        protocol += int(turn.get('tools_succeeded', 0) > 0)
        coding += int(turn.get('files_correct', False))
        local_stops = {s.get('failure') for s in streams} & LOCAL_STOPS
        failures.update(local_stops)
        deadline = bool(turn.get('timed_out')) or any(
            s.get('error_kind') == 'timeout' or s.get('failure') == 'deadline_incomplete' for s in streams)
        if deadline:
            failures.add('deadline_incomplete')
        if any(400 <= s.get('status', 0) < 500 and not s.get('failure') for s in streams):
            failures.add('request_rejected')
        if any(s.get('status', 0) >= 500 for s in streams):
            failures.add('upstream_error')
        if any(s.get('error_kind') == 'connection' or s.get('failure') == 'transport_failure' for s in streams):
            failures.add('transport_failure')
        if any(s.get('failure') == 'response_incomplete' for s in streams):
            failures.add('response_incomplete')
        if turn.get('turn_completed') and not turn.get('files_correct'):
            failures.add('coding_incorrect')
        if not turn.get('turn_completed') and not deadline and not local_stops:
            failures.add('client_incomplete')
        if any(s.get('failure') == 'response_model_mismatch' for s in streams):
            failures.add('response_model_mismatch')
        if turn.get('metadata_warning'):
            failures.add('metadata_warning')
        timings.append({'turn_ms': measurement(turn.get('elapsed_ms')), 'requests': [
            {**{key: measurement(s.get(key), integer=(key == 'status'))
              for key in ('status', 'headers_ms', 'first_delta_ms', 'completed_ms')},
             'model': identity(s, 'model', r'[A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,128}'),
             'request_id': measurement(s.get('request_id'), integer=True),
             'role': s.get('role') if s.get('role') in ('main', 'guardian') else None,
             'paid_inference': s.get('paid_inference') is True,
             'stage': s.get('stage') if s.get('stage') in ('adapter_request', 'response_headers',
                         'adapter_read', 'stream_read', 'client_write', 'completed') else None,
             'usage': {key: measurement(s.get('usage', {}).get(key), integer=True)
                       for key in ('input_tokens', 'output_tokens')}}
            for s in streams]})
    protocol += int(len(turns) == 2 and run.get('same_session', False))
    protocol += int(len(turns) == 2 and all(t.get('turn_completed') and t.get('exit_code') == 0 for t in turns))
    return {'protocol': {'score': protocol, 'maximum': 6},
            'coding': {'score': coding, 'maximum': 2,
                       'measured': sum(bool(t.get('turn_completed')) for t in turns)},
            'latency_classification': ('adapter_or_provider_wait_incomplete'
                if 'deadline_incomplete' in failures else 'observed'),
            'failures': sorted(failures), 'timing': timings,
            'file_checks': [file_checks(turn) for turn in turns],
            'scratch_settings_preserved': run.get('scratch_settings_preserved') is True}


def score(evidence):
    if not isinstance(evidence, dict) or not isinstance(evidence.get('runs', []), list):
        raise ValueError('invalid evaluation evidence')
    if len(evidence.get('runs', [])) > REPEATS:
        raise ValueError('too many evaluation repeats')
    runs = [score_run(run) for run in evidence.get('runs', [])]
    return {'report_version': 'codex-model-evaluation-v2', 'task_version': TASK_VERSION, 'rubric_version': RUBRIC_VERSION,
            'model': identity(evidence, 'model', r'[A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,128}'),
            'codex_version': identity(evidence, 'codex_version', r'codex-cli [0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?'),
            'platform': identity(evidence, 'platform', r'(?:Darwin|Linux|Windows)/(?:arm64|aarch64|x86_64|AMD64)'),
            'route': identity(evidence, 'route', r'adapted with (?:test-only loopback SSE observer|evaluation-only capped loopback observer)'),
            'started_utc': identity(evidence, 'started_utc', r'[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z'),
            'requested_repeats': REPEATS, 'completed_repeats': len(runs), 'runs': runs,
            'automatic_approval': {'status': 'not_measured', 'ordinary_task_approval_policy': 'never'},
            'normal_settings_preserved': evidence.get('normal_settings_preserved') is True,
            'status': 'incomplete', 'supported_policy_changed': False,
            **role_report(evidence, [])}


def request_cost(records, model):
    from evaluation_candidates import prices
    rates = prices(model)
    paid = [r for r in records if r.get('paid_inference', r.get('kind') != 'synthetic_task')]
    known = []
    inputs = outputs = 0
    for record in paid:
        usage = record.get('usage', {})
        incoming = measurement(usage.get('input_tokens'), integer=True)
        outgoing = measurement(usage.get('output_tokens'), integer=True)
        if incoming is not None: inputs += incoming
        if outgoing is not None: outputs += outgoing
        if incoming is not None and outgoing is not None and record.get('model', model) == model and rates:
            known.append((incoming * rates['input_usd_per_million'] + outgoing * rates['output_usd_per_million']) / 1000000)
    complete = bool(paid) and len(known) == len(paid)
    return {'model': model, 'paid_requests': len(paid), 'measured_requests': len(known),
            'input_tokens_observed': inputs, 'output_tokens_observed': outputs,
            'complete': complete, 'estimated_usd': round(sum(known), 9) if complete else None,
            'known_subtotal_usd': round(sum(known), 9) if known else None, 'prices': rates}


def role_report(evidence, approvals):
    runs = evidence.get('runs', [])
    model = identity(evidence, 'model', r'[A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,128}')
    guardian = identity(evidence, 'guardian_model', r'[A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,128}') if 'guardian_model' in evidence else model
    requests = [s for r in runs for t in r.get('turns', []) for s in t.get('streams', [])]
    def passed(run):
        result = score_run(run)
        return (result['coding']['score'] == 2 and result['protocol']['score'] == 6
                and not result['failures'] and result['scratch_settings_preserved']
                and evidence.get('normal_settings_preserved') is True
                and all(not t.get('client_error') and not t.get('output_limit') for t in run.get('turns', [])))
    main_passed = sum(passed(run) for run in runs)
    main_completed = sum(len(r.get('turns', [])) == 2 and all(t.get('turn_completed') for t in r['turns']) for r in runs)
    pairs = [approvals[i:i + 2] for i in range(0, len(approvals), 2)]
    guardian_passed = sum(len(pair) == 2 and all(a.get('passed') for a in pair) for pair in pairs)
    guardian_completed = sum(len(pair) == 2 and all(a.get('turn_completed') for a in pair) for pair in pairs)
    durations = [measurement(a.get('guardian_assessment_ms')) for a in approvals]
    def lane(attempted, completed, passed, records, model, defect=False):
        cost = request_cost(records, model)
        if defect:
            cost.update(complete=False, estimated_usd=None)
        return {'model': model, 'planned': REPEATS, 'attempted': attempted, 'completed': completed,
                'passed': passed, 'unattempted': REPEATS - attempted,
                'status': ('passed' if passed == REPEATS else 'unmeasured' if not attempted
                           else 'failed' if completed > passed else 'incomplete'),
                'cost': cost}
    return {'roles': {
        'main': lane(len(runs), main_completed, main_passed, requests, model, any(r.get('harness_defect') for r in runs)),
        'guardian': {**lane(len(pairs), guardian_completed, guardian_passed,
                           [r for a in approvals for r in a.get('requests', [])], guardian, any(a.get('harness_defect') for a in approvals)),
                     'attempted_cases': len(approvals), 'unattempted_cases': 2 * REPEATS - len(approvals),
                     'worst_assessment_ms': max(durations) if durations and all(d is not None for d in durations) else None}}}
