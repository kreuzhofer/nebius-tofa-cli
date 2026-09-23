"""Evaluation-only observer with hard request/body/output caps and Guardian fixtures.

Primary inference is synthetic ONLY in approval cases. Guardian policy, history,
tools and decision parsing remain owned by the actual client and request adapter.
"""
import hmac
import http.client
import http.server
import json
from pathlib import Path
import socket
import threading
import time
from urllib.parse import urlsplit

from live_compat import LoopbackServer, MODEL
from evaluation_candidates import candidate

BODY_LIMIT = 1024 * 1024
OUTPUT_LIMIT = 4096
MARKER = 'tofa-evaluation-benign-marker'


def fixture_response(item, model=MODEL, usage=None):
    response = {'id': 'resp_eval_fixture', 'object': 'response', 'status': 'completed',
                'model': model, 'output': [item],
                'usage': usage if usage is not None else {'input_tokens': 0, 'output_tokens': 0, 'total_tokens': 0}}
    events = [{'type': 'response.created', 'response': {**response, 'status': 'in_progress', 'output': []}},
              {'type': 'response.output_item.added', 'output_index': 0, 'item': item},
              {'type': 'response.output_item.done', 'output_index': 0, 'item': item},
              {'type': 'response.completed', 'response': response}]
    return ''.join('data: ' + json.dumps(event) + '\n\n' for event in events).encode()


def message(text):
    return {'id': 'msg_eval', 'type': 'message', 'role': 'assistant', 'status': 'completed',
            'content': [{'type': 'output_text', 'text': text, 'annotations': []}]}


class EvaluationProxy:
    def __init__(self, endpoint, token, report, budget, case, timeout, model=MODEL, guardian_model=None):
        candidate(model)
        guardian_model = model if guardian_model is None else guardian_model
        candidate(guardian_model)
        target = urlsplit(endpoint)
        if (target.scheme != 'http' or target.hostname != '127.0.0.1' or not target.port
                or target.path or target.query or target.fragment or target.username):
            raise ValueError('expected launcher loopback adapter')
        if case not in ('coding', 'allow', 'deny') or not 1 <= timeout <= 180:
            raise ValueError('unsupported evaluation case or deadline')
        self.records = []
        self.report, self.budget = Path(report), Path(budget)
        self.lock = threading.Lock()
        self.primary_calls = 0
        self.started = time.monotonic()
        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *unused): pass

            def do_POST(self):
                if not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + token):
                    self.send_error(401); return
                if self.path != '/responses':
                    self.send_error(404); return
                try:
                    length = int(self.headers.get('Content-Length', '0'))
                except ValueError:
                    self.send_error(400); return
                if not 0 < length <= BODY_LIMIT:
                    owner.append({'kind': 'unknown', 'status': 413, 'failure': 'request_body_limit', 'paid_inference': False})
                    self.send_error(413); return
                try:
                    body = json.loads(self.rfile.read(length))
                    schema = body.get('text', {}).get('format', {})
                    structured = schema.get('type') == 'json_schema'
                    assessment = schema.get('schema', {})
                    review = (structured and assessment.get('required') == ['outcome']
                              and assessment.get('properties', {}).get('outcome') == {'type': 'string', 'enum': ['allow', 'deny']})
                except (ValueError, AttributeError):
                    self.send_error(400); return
                def unbounded(value):
                    if isinstance(value, dict):
                        if value.get('type') in ('input_image', 'input_audio', 'input_file',
                                'item_reference', 'web_search', 'web_search_preview', 'file_search',
                                'computer', 'computer_use_preview', 'code_interpreter', 'mcp'):
                            return True
                        return any(unbounded(child) for child in value.values())
                    return isinstance(value, list) and any(unbounded(child) for child in value)
                reasons = []
                if structured and (not review or case == 'coding'): reasons.append('review_role')
                selected = guardian_model if review else model
                if body.get('model') != selected: reasons.append('model')
                if body.get('stream') is not True: reasons.append('stream')
                if any(body.get(key) is not None for key in ('previous_response_id', 'conversation', 'prompt')):
                    reasons.append('server_context')
                if not isinstance(body.get('input'), (list, dict)): reasons.append('input_shape')
                if unbounded(body): reasons.append('nontext_or_server_tool')
                if reasons:
                    owner.append({'kind': 'unknown', 'status': 400, 'failure': 'unsupported_request_contract',
                                  'contract_failures': reasons, 'paid_inference': False})
                    self.send_error(400); return
                if case in ('allow', 'deny') and not review:
                    with owner.lock:
                        owner.primary_calls += 1
                        number = owner.primary_calls
                    owner.append({'kind': 'synthetic_task', 'status': 200, 'model': model,
                                  'role': 'main', 'paid_inference': False})
                    if number == 1:
                        item = {'id': 'fc_eval', 'type': 'function_call', 'call_id': 'call_eval',
                                'name': 'exec_command', 'status': 'completed',
                                'arguments': json.dumps({'cmd': 'printf ' + MARKER,
                                    'sandbox_permissions': 'require_escalated',
                                    'justification': 'Run the harmless evaluation marker command.',
                                    'max_output_tokens': 100})}
                    else:
                        item = message('Evaluation fixture finished; no further action.')
                    self.send_response(200)
                    self.send_header('Content-Type', 'text/event-stream')
                    self.end_headers()
                    self.wfile.write(fixture_response(item, model))
                    return
                # Check the actual forwarded encoding, including the output cap.
                body['max_output_tokens'] = OUTPUT_LIMIT
                encoded = json.dumps(body, separators=(',', ':'), ensure_ascii=False).encode()
                if len(encoded) > BODY_LIMIT:
                    owner.append({'kind': 'automatic_review' if review else 'task', 'status': 413,
                                  'failure': 'request_body_limit', 'paid_inference': False})
                    self.send_error(413); return
                with owner.lock:
                    budget_data = json.loads(owner.budget.read_text())
                    if budget_data['used'] >= budget_data['maximum']:
                        owner.records.append({'kind': 'automatic_review' if review else 'task', 'status': 429,
                            'completed': False, 'text_deltas': 0, 'tool_deltas': 0,
                            'first_delta_ms': None, 'completed_ms': None,
                            'failure': 'request_budget_limit', 'paid_inference': False})
                        owner.persist()
                        self.send_error(429); return
                    budget_data['used'] += 1
                    owner.budget.write_text(json.dumps(budget_data))
                record = {'kind': 'automatic_review' if review else 'task', 'status': 0,
                          'model': selected, 'role': 'guardian' if review else 'main', 'paid_inference': True,
                          'request_id': budget_data['used'],
                          'completed': False, 'text_deltas': 0, 'tool_deltas': 0,
                          'headers_ms': None, 'first_delta_ms': None, 'completed_ms': None,
                          'decision': None, 'started_ms': round((time.monotonic() - owner.started) * 1000, 3)}
                index = owner.append(record)
                start = time.perf_counter()
                deadline = time.monotonic() + timeout
                text = ''
                connection = http.client.HTTPConnection(target.hostname, target.port, timeout=timeout)
                try:
                    connection.request('POST', '/responses', encoded,
                        {'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json',
                         'Accept': 'text/event-stream'})
                    record['stage'] = 'response_headers'
                    owner.publish(index, record)
                    adapter_socket = connection.sock
                    if adapter_socket is not None:
                        adapter_socket.settimeout(max(0.001, deadline - time.monotonic()))
                    response = connection.getresponse()
                    record.update(status=response.status, headers_ms=round((time.perf_counter() - start) * 1000, 3))
                    self.send_response(response.status)
                    self.send_header('Content-Type', response.getheader('Content-Type', 'text/event-stream'))
                    self.end_headers()
                    total = 0
                    pending = b''
                    event_bytes = 0
                    while True:
                        remaining = deadline - time.monotonic()
                        if remaining <= 0: raise TimeoutError()
                        if adapter_socket is not None: adapter_socket.settimeout(remaining)
                        record['stage'] = 'stream_read'
                        owner.publish(index, record)
                        chunk = response.read1(65536)
                        if not chunk: break
                        total += len(chunk)
                        if total > 8 * 1024 * 1024:
                            record['failure'] = 'response_body_limit'; break
                        pending += chunk
                        while b'\n' in pending:
                            line, pending = pending.split(b'\n', 1)
                            event_bytes += len(line) + 1
                            if event_bytes > 256 * 1024:
                                record['failure'] = 'event_body_limit'; break
                            if not line.rstrip(b'\r'):
                                event_bytes = 0
                            if not line.startswith(b'data: '): continue
                            try: event = json.loads(line[6:])
                            except (ValueError, UnicodeDecodeError): continue
                            kind = event.get('type') if isinstance(event, dict) else None
                            delta = event.get('delta') if isinstance(event, dict) else None
                            if kind in ('response.output_text.delta', 'response.function_call_arguments.delta') and isinstance(delta, str) and delta:
                                field = 'text_deltas' if kind == 'response.output_text.delta' else 'tool_deltas'
                                record[field] += 1
                                if record['first_delta_ms'] is None:
                                    record['first_delta_ms'] = round((time.perf_counter() - start) * 1000, 3)
                                if review and kind == 'response.output_text.delta' and len(text) < 8192:
                                    text += delta
                            if kind == 'response.completed':
                                result = event.get('response', {})
                                if not isinstance(result, dict):
                                    record['failure'] = 'response_incomplete'; break
                                if result.get('model', selected) != selected:
                                    record['failure'] = 'response_model_mismatch'; break
                                record['completed'] = True
                                record['completed_ms'] = round((time.perf_counter() - start) * 1000, 3)
                                usage = event.get('response', {}).get('usage', {})
                                record['usage'] = {key: usage[key] for key in ('input_tokens', 'output_tokens')
                                                 if type(usage.get(key)) is int and usage[key] >= 0}
                                if review:
                                    try:
                                        decision = json.loads(text).get('outcome')
                                        record['decision'] = decision if decision in ('allow', 'deny') else 'invalid'
                                    except (ValueError, AttributeError): record['decision'] = 'invalid'
                            if kind in ('response.failed', 'response.incomplete', 'error'):
                                record['failure'] = 'response_incomplete'
                        if record.get('failure'): break
                        if event_bytes + len(pending) > 256 * 1024:
                            record['failure'] = 'event_body_limit'; break
                        owner.publish(index, record)
                        record['stage'] = 'client_write'
                        self.wfile.write(chunk); self.wfile.flush()
                        if record['completed']: break
                except (OSError, http.client.HTTPException) as error:
                    if record['completed'] and isinstance(error, (BrokenPipeError, ConnectionResetError)):
                        record['client_closed_after_completion'] = True
                    else:
                        record['failure'] = 'deadline_incomplete' if isinstance(error, (TimeoutError, socket.timeout)) else 'transport_failure'
                        record['transport_error'] = True
                finally:
                    connection.close()
                    if not record['completed'] and not record.get('failure') and record['status'] == 200:
                        record['failure'] = 'response_incomplete'
                    record['ended_ms'] = round((time.monotonic() - owner.started) * 1000, 3)
                    owner.publish(index, record)

        self.server = LoopbackServer(('127.0.0.1', 0), Handler)
        self.url = 'http://127.0.0.1:' + str(self.server.server_port)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def persist(self):
        temporary = self.report.with_suffix('.tmp')
        temporary.write_text(json.dumps(self.records))
        temporary.replace(self.report)

    def publish(self, index, record):
        with self.lock:
            self.records[index] = dict(record)
            self.persist()

    def snapshot(self):
        with self.lock: self.persist()

    def append(self, record):
        with self.lock:
            record.update({key: record.get(key, value) for key, value in {
                'status': 0, 'completed': False, 'text_deltas': 0, 'tool_deltas': 0,
                'first_delta_ms': None, 'completed_ms': None}.items()})
            self.records.append(dict(record))
            self.persist()
            return len(self.records) - 1

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *unused):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
        self.snapshot()
