"""Opt-in, metadata-only trace of Codex requests to the launcher's local adapter.

macOS/POSIX diagnostic harness; Python 3.9+. Does not change the launcher.
"""
import argparse
from collections import Counter
import hmac
import http.client
import http.server
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import socket
import socketserver
import subprocess
import sys
import tempfile
import threading
import time
from urllib.parse import urlsplit

MODEL = 'moonshotai/Kimi-K3'
BODY_LIMIT = 16 * 1024 * 1024
RESPONSE_LIMIT = 32 * 1024 * 1024
TOOLS = {'exec_command', 'write_stdin', 'view_image', 'shell', 'shell_command',
         'apply_patch', 'update_plan', 'request_user_input', 'web', 'run', 'search_query'}
TOOL_TYPES = {'function', 'namespace', 'custom', 'web_search', 'web_search_preview',
              'local_shell', 'computer', 'computer_use_preview'}
ITEM_TYPES = {'additional_tools', 'message', 'function_call', 'function_call_output', 'custom_tool_call',
              'custom_tool_call_output', 'reasoning', 'item_reference', 'local_shell_call',
              'web_search_call', 'computer_call', 'computer_call_output'}
EVENTS = {'response.' + suffix for suffix in (
    'created', 'in_progress', 'completed', 'failed', 'incomplete', 'queued',
    'output_item.added', 'output_item.done', 'content_part.added', 'content_part.done',
    'output_text.delta', 'output_text.done', 'function_call_arguments.delta',
    'function_call_arguments.done', 'reasoning_summary_text.delta',
    'reasoning_summary_text.done', 'reasoning_summary_part.added',
    'reasoning_summary_part.done', 'refusal.delta', 'refusal.done')}
EVENTS.add('error')
CONFLICT = b'Cannot combine tool calls with constrained decoding'


def category(value, allowed):
    return value if isinstance(value, str) and value in allowed else 'other'


def request_metadata(body):
    """Allowlist categorical values; never serialize arbitrary external strings."""
    try:
        value = json.loads(body)
    except (ValueError, UnicodeDecodeError, RecursionError):
        return {'valid_json': False}
    if not isinstance(value, dict):
        return {'valid_json': False}
    tools = []

    def collect(entries, source, depth=0):
        if not isinstance(entries, list) or depth > 2:
            return
        for tool in entries[:100]:
            if not isinstance(tool, dict) or len(tools) >= 100:
                continue
            entry = {'type': category(tool.get('type'), TOOL_TYPES)}
            if 'name' in tool:
                entry['name'] = category(tool['name'], TOOLS | {'functions'})
            if source != 'tools':
                entry['source'] = source
            tools.append(entry)
            collect(tool.get('tools'), 'namespace', depth + 1)

    collect(value.get('tools'), 'tools')
    inputs = value.get('input', [])
    if isinstance(inputs, dict):
        collect(inputs.get('additional_tools'), 'input.additional_tools')
        inputs = inputs.get('items', [])
    counts = Counter()
    roles = Counter()
    if isinstance(inputs, list):
        for item in inputs:
            if isinstance(item, dict):
                if item.get('type') == 'additional_tools':
                    collect(item.get('tools'), 'input.additional_tools')
                counts[category(item.get('type', 'message'), ITEM_TYPES)] += 1
                if 'role' in item:
                    roles[category(item['role'], {'user', 'assistant', 'system', 'developer', 'tool'})] += 1
    text = value.get('text')
    fmt = text.get('format') if isinstance(text, dict) else None
    format_summary = None
    if isinstance(fmt, dict):
        format_summary = {'type': category(fmt.get('type'), {'text', 'json_object', 'json_schema'})}
        if isinstance(fmt.get('strict'), bool):
            format_summary['strict'] = fmt['strict']
    choice = value.get('tool_choice')
    return {'model': category(value.get('model'), {MODEL, 'codex-auto-review'}),
            'tools': tools, 'text_format': format_summary,
            'tool_choice': category(choice, {'auto', 'none', 'required'}) if not isinstance(choice, dict)
            else category(choice.get('type'), TOOL_TYPES | {'allowed_tools'}),
            'input_types': dict(counts), 'input_roles': dict(roles)}


class LoopbackServer(http.server.ThreadingHTTPServer):
    daemon_threads = False

    def server_bind(self):
        socketserver.TCPServer.server_bind(self)
        self.server_name, self.server_port = self.server_address

    def handle_error(self, *args):
        # HTTP exceptions can embed private request content or addresses.
        pass


class TraceProxy:
    """Authenticated bounded /responses observer; target must be numeric loopback."""
    def __init__(self, endpoint, token, output, max_requests=24, timeout=300):
        target = urlsplit(endpoint)
        if (target.scheme != 'http' or target.hostname != '127.0.0.1' or not target.port
                or target.path or target.query or target.fragment or target.username or target.password):
            raise ValueError('expected launcher loopback adapter endpoint')
        if not token or not 1 <= max_requests <= 100 or not 1 <= timeout <= 300:
            raise ValueError('invalid trace limits or local token')
        self.lock = threading.Lock()
        self.count = 0
        self.stream = os.fdopen(os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w')
        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args): pass

            def setup(self):
                super().setup()
                self.connection.settimeout(min(timeout, 5))

            def reject(self, status):
                self.send_response(status)
                self.send_header('Content-Length', '0')
                self.end_headers()

            def do_POST(self):
                supplied = self.headers.get('Authorization', '').encode('utf-8')
                if not hmac.compare_digest(supplied, ('Bearer ' + token).encode('utf-8')):
                    return self.reject(401)
                if self.path != '/responses':
                    return self.reject(404)
                lengths = self.headers.get_all('Content-Length', [])
                if self.headers.get('Transfer-Encoding') or len(lengths) != 1 or not lengths[0].isdigit():
                    return self.reject(400)
                length = int(lengths[0])
                if not 0 < length <= BODY_LIMIT:
                    return self.reject(413)
                with owner.lock:
                    if owner.count >= max_requests:
                        return self.reject(429)
                    owner.count += 1
                    number = owner.count
                start = time.perf_counter()
                connection = http.client.HTTPConnection('127.0.0.1', target.port, timeout=timeout)
                record = {'phase': 'response', 'request_id': number, 'status': 0, 'events': {}}
                headers_sent = False
                stage = 'observer_receive'
                try:
                    body = self.rfile.read(length)
                    if len(body) != length:
                        raise ConnectionError('incomplete body')
                    owner.write({'phase': 'request', 'request_id': number, 'request': request_metadata(body)})
                    stage = 'adapter_send'
                    owner.write({'phase': 'stage', 'request_id': number, 'stage': stage})
                    connection.request('POST', '/responses', body,
                        {'Authorization': 'Bearer ' + token, 'Content-Type': self.headers.get('Content-Type', 'application/json'),
                         'Accept': self.headers.get('Accept', 'text/event-stream')})
                    adapter_socket = connection.sock
                    if adapter_socket is not None:
                        adapter_socket.settimeout(max(0.001, timeout - (time.perf_counter() - start)))
                    stage = 'response_headers'
                    owner.write({'phase': 'stage', 'request_id': number, 'stage': stage})
                    response = connection.getresponse()
                    record['status'] = response.status
                    record['headers_ms'] = round((time.perf_counter() - start) * 1000, 3)
                    owner.write({'phase': 'headers', 'request_id': number, 'status': response.status,
                                 'elapsed_ms': record['headers_ms']})
                    self.send_response(response.status)
                    self.send_header('Content-Type', response.getheader('Content-Type', 'application/octet-stream'))
                    self.end_headers()
                    headers_sent = True
                    pending = b''
                    tail = b''
                    total = 0
                    counts = Counter()
                    event_rows = 0
                    is_sse = response.getheader('Content-Type', '').split(';')[0] == 'text/event-stream'
                    while True:
                        if time.perf_counter() - start > timeout:
                            record['error'] = 'deadline'
                            break
                        stage = 'stream_read'
                        if adapter_socket is not None:
                            adapter_socket.settimeout(max(0.001, timeout - (time.perf_counter() - start)))
                        chunk = response.read1(65536)
                        if not chunk:
                            break
                        total += len(chunk)
                        if total > RESPONSE_LIMIT:
                            record['error'] = 'response_limit'
                            break
                        if response.status >= 400 and CONFLICT in tail + chunk:
                            record['error_class'] = 'tools_schema_conflict'
                        tail = chunk[-len(CONFLICT):]
                        stage = 'client_write'
                        self.wfile.write(chunk)
                        self.wfile.flush()
                        if is_sse:
                            pending += chunk
                            while b'\n' in pending:
                                line, pending = pending.split(b'\n', 1)
                                if line.startswith(b'data:'):
                                    if line[5:].strip() == b'[DONE]':
                                        counts['done_marker'] += 1
                                        continue
                                    try:
                                        event = json.loads(line[5:])
                                    except (ValueError, UnicodeDecodeError, RecursionError):
                                        continue
                                    if isinstance(event, dict):
                                        kind = category(event.get('type'), EVENTS)
                                        counts[kind] += 1
                                        if kind in {'error', 'response.failed', 'response.incomplete'} and CONFLICT in line:
                                            record['error_class'] = 'tools_schema_conflict'
                                        if event_rows < 128:
                                            owner.write({'phase': 'event', 'request_id': number, 'type': kind,
                                                'elapsed_ms': round((time.perf_counter() - start) * 1000, 3)})
                                            event_rows += 1
                                        else:
                                            record['event_rows_limited'] = True
                            if len(pending) > 256 * 1024:
                                record['events_truncated'] = True
                                pending = b''
                        record['events'] = dict(counts)
                except (OSError, http.client.HTTPException, ValueError) as error:
                    record['error'] = 'timeout' if isinstance(error, (TimeoutError, socket.timeout)) else 'connection'
                    record['stage'] = stage
                    if not headers_sent:
                        try:
                            self.reject(502)
                        except OSError:
                            pass
                finally:
                    connection.close()
                    record['elapsed_ms'] = round((time.perf_counter() - start) * 1000, 3)
                    owner.write(record)

        try:
            self.server = LoopbackServer(('127.0.0.1', 0), Handler)
        except BaseException:
            self.stream.close()
            raise
        self.port = self.server.server_port

    def write(self, value):
        with self.lock:
            self.stream.write(json.dumps(value, separators=(',', ':')) + '\n')
            self.stream.flush()

    def __enter__(self):
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        return self

    def __exit__(self, *args):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
        self.stream.close()


PROBE = """Use exec_command exactly once to run `printf tofa-approval-probe` with
sandbox_permissions="require_escalated" and justification "Run a harmless printf approval probe."
This explicitly tests the automatic approval reviewer. If approval fails, report
that briefly and stop without retrying. Do not inspect files, environment variables,
credentials, or the network. Do not install anything or modify any files."""


def observe(args):
    index = next(i for i, arg in enumerate(args) if arg.startswith('model_providers.nebius-tofa='))
    match = re.search(r'base_url\s*=\s*("[^"]+")', args[index])
    if match is None:
        raise ValueError('launcher adapter configuration missing')
    token = os.environ['TOFA_API_KEY']
    env = {key: os.environ[key] for key in ('PATH', 'TMPDIR', 'LANG', 'LC_ALL') if key in os.environ}
    env.update(HOME=os.environ['TOFA_TRACE_HOME'], CODEX_HOME=os.environ['TOFA_TRACE_CODEX_HOME'],
               TOFA_API_KEY=token, OTEL_SDK_DISABLED='true')
    with TraceProxy(json.loads(match.group(1)), token, os.environ['TOFA_TRACE_OUTPUT'],
                    timeout=int(os.environ['TOFA_TRACE_REQUEST_TIMEOUT'])) as proxy:
        args[index] = args[index][:match.start(1)] + json.dumps(f'http://127.0.0.1:{proxy.port}') + args[index][match.end(1):]
        return subprocess.call([os.environ['TOFA_TRACE_CODEX']] + args, env=env)


def stop(process):
    try:
        os.killpg(process.pid, signal.SIGTERM)
        process.wait(timeout=3)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=3)
    except ProcessLookupError:
        pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--launcher', default=shutil.which('tofa'), help='launcher with saved login')
    parser.add_argument('--codex', default=shutil.which('codex'), help='actual installed Codex binary')
    parser.add_argument('--output', required=True, help='new private JSONL file; never overwritten')
    parser.add_argument('--timeout', type=int, default=600, help='diagnostic session budget, seconds (1-600)')
    parser.add_argument('--request-timeout', type=int, default=300, help='total diagnostic budget per adapter request, seconds (1-300)')
    choice = parser.add_mutually_exclusive_group(required=True)
    choice.add_argument('--approval-probe', action='store_true', help='harmless printf automatic approval probe')
    choice.add_argument('--prompt', help='one scratch Codex task; no persistent client configuration')
    options = parser.parse_args()
    if os.name != 'posix':
        parser.error('this diagnostic harness supports macOS/POSIX only')
    if not 1 <= options.timeout <= 600 or not 1 <= options.request_timeout <= 300:
        parser.error('deadline outside supported range')
    if not options.launcher or not options.codex:
        parser.error('installed launcher and Codex are required')
    output = Path(options.output).absolute()
    if os.path.lexists(output):
        parser.error('trace output must not exist')
    launcher = str(Path(options.launcher).absolute())
    codex = str(Path(options.codex).absolute())
    with tempfile.TemporaryDirectory(prefix='tofa-trace-') as directory:
        root = Path(directory)
        for part in ('bin', 'home', 'codex', 'workspace'):
            (root / part).mkdir(mode=0o700)
        (root / 'codex' / 'config.toml').write_text('allow_login_shell = false\n')
        shim = root / 'bin' / 'codex'
        shim.write_text('#!/bin/sh\nexec ' + shlex.quote(sys.executable) + ' ' +
                        shlex.quote(str(Path(__file__).resolve())) + ' --observe "$@"\n')
        shim.chmod(0o700)
        # Launcher retains its normal HOME and saved login. Only actual Codex is isolated.
        env = dict(os.environ, PATH=str(root / 'bin') + os.pathsep + os.environ.get('PATH', ''),
                   TOFA_TRACE_HOME=str(root / 'home'), TOFA_TRACE_CODEX_HOME=str(root / 'codex'),
                   TOFA_TRACE_CODEX=codex, TOFA_TRACE_OUTPUT=str(output),
                   TOFA_TRACE_REQUEST_TIMEOUT=str(options.request_timeout))
        command = [launcher, 'launch', 'codex', '--model', MODEL, '--allow-unverified', '--']
        if not options.approval_probe:
            command.extend(['--sandbox', 'read-only'])
        command.extend(['exec', '--skip-git-repo-check', '--ignore-rules', '--json'])
        if options.approval_probe:
            command.append('--approve-for-me')
        command.append('-')
        print('Trace started; JSONL records flush as requests progress.', flush=True)
        timed_out = False
        process = subprocess.Popen(command, cwd=root / 'workspace', env=env,
                                   stdin=subprocess.PIPE, stdout=subprocess.DEVNULL,
                                   stderr=subprocess.DEVNULL, start_new_session=True)
        try:
            process.communicate((PROBE if options.approval_probe else options.prompt).encode(), timeout=options.timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            stop(process)
        except BaseException:
            stop(process)
            raise
        print(json.dumps({'phase': 'session', 'exit_code': process.returncode, 'timed_out': timed_out,
                          'trace_created': output.is_file()}), flush=True)
        return 124 if timed_out else process.returncode if process.returncode else (0 if output.is_file() else 1)


if __name__ == '__main__':
    try:
        raise SystemExit(observe(sys.argv[2:]) if sys.argv[1:2] == ['--observe'] else main())
    except (OSError, ValueError, KeyError, StopIteration):
        # Never surface paths, credentials, provider config, or raw exception text.
        print('Trace harness failed during setup or local IO.', file=sys.stderr)
        raise SystemExit(1)
