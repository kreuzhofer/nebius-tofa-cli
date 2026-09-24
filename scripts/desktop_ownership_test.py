"""Opt-in native desktop ownership checks using disposable synthetic profiles.

TOFA_TEST_DESKTOP_APP=/tmp/tofa47-qualified/ChatGPT.app \\
    python3 scripts/desktop_ownership_test.py -v

The installed executable and engine run with mock Keychain and dummy file-backed
authentication. No ordinary user profile, real credential, or inference is used.
These checks qualify native ownership, not desktop UI/account continuity.
TOFA_TEST_DESKTOP_OWNERSHIP_EVIDENCE optionally names a sanitized JSON output,
including failed qualification results.
"""

import hashlib
import json
import os
from pathlib import Path
import plistlib
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest


APP = os.environ.get("TOFA_TEST_DESKTOP_APP")
COLLISION_TIMEOUT = 5
QUALIFIED_VERSION = "26.917.71314"
QUALIFIED_BUILD = "10954"
QUALIFIED_ENGINE = "codex-cli 0.155.0-alpha.16.4"
QUALIFIED_FILES = {
    "Contents/Resources/app.asar":
        "03108a728bdb1616958ab89587c5495cab0cf4cd1bbe109bdfb186df0a113804",
    "Contents/Resources/codex":
        "93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5",
    "Contents/Frameworks/Codex Framework.framework/Versions/153.0.8010.53/Codex Framework":
        "fac56b9423fe81e6c206a8ca4e755d5dfbc698bef7bbbb3a4fc19cfa6c3e6ca2",
}


@unittest.skipUnless(sys.platform == "darwin" and APP,
                     "set TOFA_TEST_DESKTOP_APP on macOS to run installed-app checks")
class DesktopOwnershipTests(unittest.TestCase):
    observations = {}

    @classmethod
    def verify_bundle(cls):
        app = Path(APP).resolve()
        with (app / "Contents" / "Info.plist").open("rb") as source:
            metadata = plistlib.load(source)
        identity = (metadata["CFBundleShortVersionString"], metadata["CFBundleVersion"])
        if identity != (QUALIFIED_VERSION, QUALIFIED_BUILD):
            raise AssertionError("installed-app qualification requires the pinned client snapshot")
        for name, expected in QUALIFIED_FILES.items():
            digest = hashlib.sha256()
            with (app / name).open("rb") as source:
                for chunk in iter(lambda: source.read(1024 * 1024), b""):
                    digest.update(chunk)
            if digest.hexdigest() != expected:
                raise AssertionError("qualified bundle changed: " + name)

    @classmethod
    def setUpClass(cls):
        cls.verify_bundle()

    @classmethod
    def tearDownClass(cls):
        cls.verify_bundle()

    def setUp(self):
        # Keep app-server Unix socket paths below macOS's pathname limit.
        self.temporary = tempfile.TemporaryDirectory(prefix="tofa-owner-", dir="/tmp")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.home = self.root / "home"
        self.codex = self.home / ".codex"
        self.codex.mkdir(parents=True)
        self.profile = self.home / "Library" / "Application Support" / "Codex"
        self.profile.mkdir(parents=True)
        self.app = Path(APP).resolve()
        with (self.app / "Contents" / "Info.plist").open("rb") as source:
            metadata = plistlib.load(source)
        self.executable = self.app / "Contents" / "MacOS" / metadata["CFBundleExecutable"]
        self.engine = self.app / "Contents" / "Resources" / "codex"
        self.assertTrue(self.executable.is_file())
        self.assertTrue(self.engine.is_file())
        (self.codex / "auth.json").write_text(
            json.dumps({"OPENAI_API_KEY": "synthetic-ownership-fixture"}))
        (self.codex / "config.toml").write_text(
            'cli_auth_credentials_store="file"\n'
            'openai_base_url="http://127.0.0.1:0"\n'
            '[features]\nanalytics=false\n')
        observer = self.root / "engine-observer"
        observer.write_text(
            f"#!{sys.executable}\n"
            "import json, os, sys\n"
            "if 'app-server' in sys.argv[1:]:\n"
            "    with open(os.environ['TOFA_OWNERSHIP_ENGINE_EVENTS'], 'a') as out:\n"
            "        out.write(json.dumps({'pid': os.getpid(), "
            "'parent_pid': os.getppid()}) + '\\n')\n"
            f"os.execv({str(self.engine)!r}, [{str(self.engine)!r}, *sys.argv[1:]])\n")
        observer.chmod(0o700)
        self.environment = {
            "HOME": str(self.home),
            "CFFIXED_USER_HOME": str(self.home),
            "CODEX_HOME": str(self.codex),
            "ZDOTDIR": str(self.home),
            "CODEX_CLI_PATH": str(observer),
            "CODEX_SPARKLE_ENABLED": "false",
            "PATH": "/usr/bin:/bin:/usr/sbin:/sbin",
            "TMPDIR": str(self.root),
            "OTEL_SDK_DISABLED": "true",
        }
        version = subprocess.check_output(
            [str(self.engine), "--version"], env=self.environment,
            cwd=self.root, text=True, timeout=5).strip()
        self.assertEqual(version, QUALIFIED_ENGINE)
        foundation_probe = (
            'ObjC.import("Foundation"); '
            'ObjC.unwrap($.NSSearchPathForDirectoriesInDomains('
            '$.NSApplicationSupportDirectory,$.NSUserDomainMask,true).firstObject)')
        support = subprocess.check_output(
            ["/usr/bin/osascript", "-l", "JavaScript", "-e", foundation_probe],
            env=self.environment, text=True, timeout=10).strip()
        self.assertEqual(Path(support).resolve(), self.profile.parent)
        self.children = []
        self.addCleanup(self.stop_owned_processes)

    def launch(self, explicit_profile=False):
        index = len(self.children)
        events = self.root / f"engine-{index}.jsonl"
        log = self.root / f"desktop-{index}.log"
        environment = dict(self.environment, TOFA_OWNERSHIP_ENGINE_EVENTS=str(events))
        command = [str(self.executable), "--use-mock-keychain"]
        if explicit_profile:
            environment["CODEX_ELECTRON_USER_DATA_PATH"] = str(self.profile)
            command.append("--user-data-dir=" + str(self.profile))
        with log.open("w") as output:
            process = subprocess.Popen(
                command, cwd=self.root, env=environment, stdout=output,
                stderr=output, start_new_session=True)
        child = {"process": process, "events": events, "log": log}
        self.children.append(child)
        return child

    def owner_pid(self):
        lock = self.profile / "SingletonLock"
        try:
            target = os.readlink(lock)
        except FileNotFoundError:
            return None
        hostname, separator, pid = target.rpartition("-")
        self.assertTrue(separator and pid.isdecimal(), "invalid native singleton owner")
        self.assertEqual(hostname, socket.gethostname())
        return int(pid)

    def engine_events(self, child):
        if not child["events"].exists():
            return []
        return [json.loads(line) for line in child["events"].read_text().splitlines()]

    def wait_for_owner(self, child, require_native_owner=True):
        process = child["process"]
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            self.assertIsNone(process.poll(), "desktop exited before engine startup")
            events = self.engine_events(child)
            owner_pid = self.owner_pid()
            if require_native_owner and events and owner_pid != process.pid:
                observation = self.observations.setdefault(self._testMethodName, {})
                observation.update(
                    qualified=False, winner_engine_started=True,
                    native_owner_matches_spawned_desktop=False,
                    native_lock_present=owner_pid is not None)
                self.fail("engine started without native lock identifying its desktop")
            if events:
                if require_native_owner:
                    for name in ("SingletonLock", "SingletonSocket", "SingletonCookie"):
                        self.assertTrue((self.profile / name).is_symlink(), name)
                for event in events:
                    self.assertEqual(event["parent_pid"], process.pid)
                    executable = subprocess.run(
                        ["/bin/ps", "-ww", "-p", str(event["pid"]), "-o", "comm="],
                        capture_output=True, text=True, timeout=3)
                    if executable.returncode or executable.stdout.strip() != str(self.engine):
                        break
                else:
                    log = child["log"].read_text()
                    self.assertIn("enableUpdater=false", log)
                    self.assertIn("enableSparkle=false", log)
                    return
            time.sleep(0.05)
        self.fail("desktop did not establish both native PID ownership and engine startup")

    def assert_reused_owner(self, owner, contender):
        try:
            code = contender["process"].wait(timeout=COLLISION_TIMEOUT)
        except subprocess.TimeoutExpired:
            self.fail("contender exceeded safe collision deadline; stopping owned groups")
        self.assertEqual(code, 0)
        self.assertIsNone(owner["process"].poll())
        self.assertEqual(self.owner_pid(), owner["process"].pid)
        self.assertEqual(self.engine_events(contender), [])
        notified = "Opening in existing browser session." in contender["log"].read_text()
        if code == 0:
            self.assertTrue(notified)
        self.observations[self._testMethodName] = {
            "qualified": True,
            "native_owner_matches_spawned_desktop": True,
            "contender_exit_code": code,
            "contender_started_engine": False,
            "existing_browser_notification": notified,
        }

    def test_ordinary_launch_reuses_existing_ordinary_owner(self):
        owner = self.launch()
        self.wait_for_owner(owner)
        self.assert_reused_owner(owner, self.launch())

    def test_ordinary_launch_reuses_explicit_same_profile_owner(self):
        owner = self.launch(explicit_profile=True)
        self.wait_for_owner(owner)
        self.assert_reused_owner(owner, self.launch())

    def test_explicit_same_profile_launch_reuses_ordinary_owner(self):
        owner = self.launch()
        self.wait_for_owner(owner)
        self.assert_reused_owner(owner, self.launch(explicit_profile=True))

    def test_graceful_quit_records_artifacts_and_allows_native_relaunch(self):
        owner = self.launch()
        self.wait_for_owner(owner)
        # Engine spawn precedes the user-visible window. Qualify a user-equivalent
        # quit only after the installed app reports its primary window ready;
        # terminate() reports request delivery, not completed termination.
        ready_marker = "window ready-to-show appearance=primary"
        deadline = time.monotonic() + 10
        while ready_marker not in owner["log"].read_text():
            self.assertIsNone(owner["process"].poll(), "desktop exited before window readiness")
            if time.monotonic() >= deadline:
                self.fail("primary window did not become ready within 10 seconds")
            time.sleep(0.05)
        pid = owner["process"].pid
        self.assertEqual(self.owner_pid(), pid)
        quit_script = (
            'ObjC.import("AppKit"); '
            f'const app = $.NSRunningApplication.runningApplicationWithProcessIdentifier({pid}); '
            f'if (Number(app.processIdentifier) !== {pid}) throw new Error("Owned PID missing"); '
            'JSON.stringify({accepted: Boolean(app.terminate)})')
        response = subprocess.check_output(
            ["/usr/bin/osascript", "-l", "JavaScript", "-e", quit_script],
            env=self.environment, text=True, timeout=5)
        self.assertTrue(json.loads(response)["accepted"])
        code = owner["process"].wait(timeout=8)
        remaining = [name for name in ("SingletonLock", "SingletonSocket", "SingletonCookie")
                     if os.path.lexists(self.profile / name)]
        self.observations[self._testMethodName] = {
            "primary_window_ready_before_quit": True,
            "owned_pid_native_termination_requested": True,
            "graceful_exit_code": code,
            "native_artifacts_after_graceful_quit": remaining,
        }
        self.assertEqual(code, 0)
        if "SingletonLock" in remaining:
            self.assertEqual(self.owner_pid(), pid)
        self.wait_for_owner(self.launch())
        self.observations[self._testMethodName]["relaunch_started_owned_engine"] = True

    def test_simultaneous_launches_expose_ownership_evidence(self):
        first = self.launch()
        second = self.launch()
        deadline = time.monotonic() + COLLISION_TIMEOUT
        while time.monotonic() < deadline:
            exited = [child for child in (first, second)
                      if child["process"].poll() is not None]
            if exited:
                self.assertEqual(len(exited), 1)
                loser = exited[0]
                winner = second if loser is first else first
                self.wait_for_owner(winner, require_native_owner=False)
                self.assertEqual(self.engine_events(loser), [])
                self.assertLessEqual(sum(bool(self.engine_events(child))
                                         for child in (first, second)), 1)
                matches = self.owner_pid() == winner["process"].pid
                self.observations[self._testMethodName] = {
                    "native_ownership_qualified": matches,
                    "launcher_must_refuse_missing_owner_evidence": not matches,
                    "at_most_one_engine_started": True,
                    "winner_engine_started": True,
                    "native_owner_matches_spawned_desktop": matches,
                    "native_lock_present": self.owner_pid() is not None,
                    "contender_exit_code": loser["process"].returncode,
                    "contender_started_engine": False,
                    "native_singleton_creation_error": (
                        "Failed to create a ProcessSingleton" in loser["log"].read_text()),
                }
                return
            time.sleep(0.05)
        self.fail("concurrent launches exceeded safe collision deadline")

    def stop_owned_processes(self):
        for child in self.children:
            try:
                os.killpg(child["process"].pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        for child in self.children:
            process = child["process"]
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                pass
        observation = self.observations.get(self._testMethodName)
        if observation is not None:
            observation["native_artifacts_after_sigterm"] = [
                name for name in ("SingletonLock", "SingletonSocket", "SingletonCookie")
                if os.path.lexists(self.profile / name)]
            observation["owned_desktops_exited_after_sigterm"] = all(
                child["process"].poll() is not None for child in self.children)
        for child in self.children:
            process = child["process"]
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=3)


if __name__ == "__main__":
    program = unittest.main(exit=False)
    evidence = os.environ.get("TOFA_TEST_DESKTOP_OWNERSHIP_EVIDENCE")
    if evidence and DesktopOwnershipTests.observations:
        Path(evidence).write_text(json.dumps({
            "successful": program.result.wasSuccessful(),
            "tests_run": program.result.testsRun,
            "app_version": QUALIFIED_VERSION,
            "app_build": QUALIFIED_BUILD,
            "engine_version": QUALIFIED_ENGINE,
            "bundle_sha256": QUALIFIED_FILES,
            "updater_disabled_in_test_processes": True,
            "observations": DesktopOwnershipTests.observations,
        }, indent=2) + "\n")
    sys.exit(not program.result.wasSuccessful())
