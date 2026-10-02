"""Explicit reuse of an initialized Windows sandbox for disposable qualification.

Native sandbox setup is tied to CODEX_HOME. Keep its runtime state in place,
apply test settings only through argv, and remove only sessions observed by this
run. This module never initializes a sandbox or copies credentials.
"""
import csv
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

HOME_VARIABLE = 'TOFA_NATIVE_WINDOWS_CODEX_HOME'
SESSIONS_VARIABLE = 'TOFA_QUALIFICATION_SESSION_IDS'


def native_home():
    value = os.environ.get(HOME_VARIABLE)
    if not value:
        return None
    home = Path(value)
    if os.name != 'nt' or not home.is_absolute() or not home.is_dir():
        raise ValueError('native Windows sandbox home must be an existing absolute directory on Windows')
    return home.resolve()


def prepare_workspace(workspace):
    # Python's private Windows temporary tree inherits OWNER RIGHTS. A file
    # created by the sandbox account otherwise becomes unreadable to the runner.
    result = subprocess.run(['whoami.exe', '/user', '/fo', 'csv', '/nh'],
                            capture_output=True, text=True, check=True, timeout=10)
    sid = next(csv.reader(result.stdout.splitlines()))[1]
    if not re.fullmatch(r'S-\d+(?:-\d+)+', sid):
        raise ValueError('invalid current-user SID')
    subprocess.run(['icacls.exe', str(workspace), '/grant:r', '*' + sid + ':(OI)(CI)M'],
                   capture_output=True, check=True, timeout=10)


def client_arguments(args):
    if native_home() is None:
        return args
    import tomllib  # Windows qualification with shared sandbox state requires Python 3.11+.
    private = Path(os.environ['TOFA_LIVE_CODEX_HOME'])
    extra = ['-c', 'mcp_servers={}', '-c', 'plugins={}', '-c', 'hooks={}',
             '-c', 'features.plugins=false', '-c', 'features.hooks=false',
             '-c', 'features.memories=false', '-c', 'history.persistence="none"',
             '-c', 'project_doc_max_bytes=0',
             '-c', 'log_dir=' + json.dumps(str(private / 'logs')),
             '-c', 'sqlite_home=' + json.dumps(str(private / 'sqlite'))]
    def append_config(data, prefix=()):
        for key, value in data.items():
            # The exec fixture uses --skip-git-repo-check. Do not add trust
            # entries to the native profile or encode paths as dotted keys.
            if key == 'projects':
                continue
            if not re.fullmatch(r'[A-Za-z_][A-Za-z_0-9]*', key):
                raise ValueError('invalid private qualification setting')
            path = prefix + (key,)
            if isinstance(value, dict):
                append_config(value, path)
            else:
                extra.extend(['-c', '.'.join(path) + '=' + json.dumps(value)])
    append_config(tomllib.loads((private / 'config.toml').read_text()))
    return extra + args


def record_sessions(identities):
    destination = os.environ.get(SESSIONS_VARIABLE)
    if not destination:
        return
    with Path(destination).open('a', encoding='utf-8') as stream:
        for identity in identities:
            if not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', identity):
                raise ValueError('invalid qualification session identity')
            stream.write(identity + '\n')


class NativeState:
    def __init__(self, root):
        self.home = native_home()
        self.old_sessions = os.environ.get(SESSIONS_VARIABLE)
        if self.home is None:
            return
        self.watched = [self.home / name for name in ('config.toml', 'auth.json')]
        self.before = self.snapshot()
        self.sessions = root / 'owned-native-sessions.txt'
        os.environ[SESSIONS_VARIABLE] = str(self.sessions)

    def snapshot(self):
        return [hashlib.sha256(path.read_bytes()).digest() if path.is_file() else None for path in self.watched]

    def finish(self, report):
        if self.home is None:
            return
        preserved, cleaned, identities = False, False, set()
        try:
            preserved = self.snapshot() == self.before
            identities = set(self.sessions.read_text().splitlines()) if self.sessions.exists() else set()
            for identity in identities:
                if not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', identity):
                    raise ValueError('invalid qualification session identity')
                files = list((self.home / 'sessions').glob('**/rollout-*-' + identity + '.jsonl'))
                files += list((self.home / 'shell_snapshots').glob(identity + '.*'))
                for path in files:
                    if path.is_symlink() or not path.resolve().is_relative_to(self.home):
                        raise ValueError('unexpected native session symlink')
                    path.unlink()
            cleaned = bool(identities) if report['passed'] else True
        except (OSError, ValueError):
            pass
        finally:
            if self.old_sessions is None:
                os.environ.pop(SESSIONS_VARIABLE, None)
            else:
                os.environ[SESSIONS_VARIABLE] = self.old_sessions
            report.update(native_sandbox_home_reused=True,
                          native_config_auth_preserved=preserved,
                          owned_native_sessions_removed=cleaned,
                          owned_native_session_count=len(identities))
            report['passed'] = report['passed'] and preserved and cleaned
