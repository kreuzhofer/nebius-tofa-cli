# Windows ARM64 CLI selection validation — 2026-09-25

Issue [#56](https://github.com/kreuzhofer/nebius-tofa-cli/issues/56) was validated
against installed Codex CLI 0.156.1 on Windows 11 Pro ARM64 (build 26200), using
the SabreTest account in a local UTM VM. The source candidate was `16f0a35` plus
the test portability changes in the commit containing this report.

These checks use synthetic responses from a local HTTP server. They establish
launcher/client behavior, not compatibility with live Token Factory models.
No verification record was promoted to supported. Desktop clients and Claude
were outside this CLI-selection validation.

## Installed-client results

All 54 top-level tests in `./internal/tofa` passed natively, including:

- Tool execution and continued conversation through the request adapter.
- Default Guardian, explicit distinct Guardian, and same-model allow/deny gates.
- A Guardian tool check followed by continued review and allowed execution.
- Invalid or missing review fields, provider failure, and cancelled review.
- Native Windows process and credential-file access-control tests.

The test executable was built on macOS with Go 1.27.1 using
`GOOS=windows GOARCH=arm64 go test -c ./internal/tofa`, then run in Windows with
`TOFA_TEST_CODEX` pointing to the installed ARM64 executable and
`-test.v -test.timeout=5m`. The full package completed in 61.1 seconds.
The [machine-readable evidence](evidence/windows-arm64-cli-selection-2026-09-25.json)
records counts and hashes of the executable and retained host log.

The first native run exposed three fixture assumptions:

1. `printf` was replaced with portable `echo` commands.
2. Successful execution events are decoded as JSON and checked for completed
   status, zero exit code, and the exact output marker after line-ending trimming.
3. An isolated Codex home has no Windows sandbox backend configured. The fixtures
   now pass `windows.sandbox="unelevated"` to Windows child processes, retaining
   read-only execution and native approval enforcement. No real client config is
   changed. This matches Codex's
   [0.156.1 policy tests](https://github.com/openai/codex/blob/rust-v0.156.1/codex-rs/core/src/exec_policy_windows_tests.rs),
   which reject unmatched commands under a read-only policy when the backend is
   disabled. Environment-name casing and temporary-directory hypotheses were
   tested, did not resolve the failures, and were not retained.

## Additional validation

Nine native Windows process/discovery tests and the offline PowerShell installer
lifecycle passed. The real ARM64 candidate installer lifecycle also passed under
PowerShell 7, including fresh PATH discovery, repeated install, asynchronous
uninstall, retained synthetic login, reinstall, and purge. The test account's
persistent PATH was verified unchanged after each lifecycle run.

The qualification fixture's hard-coded AMD64 asset names were changed to use
its existing native-architecture detection. The broader qualification runner remains **failed**: 17 tests ran with 12
failures and one error. Its native Python process downloads ARM64, but the
installer's Windows PowerShell 5.1 child reports `PROCESSOR_ARCHITECTURE=AMD64`
with no `PROCESSOR_ARCHITEW6432`, then requests an absent AMD64 asset. Assertions
for later stages fail before reaching their intended scenario. The actual
candidate/helper supervision and PowerShell module-path tests passed. This
separate installer architecture issue is not repaired by the CLI-selection
fixture changes; it remains a release-qualification gap. Counts and log hashes
are in the accompanying evidence file.

The full macOS Go race suite passed. Installed-client tests passed again after
the final fixture change, and both macOS and Windows ARM64 `go vet` passed.
The macOS offline Python checks passed 110 tests, with 41 platform/client
skips. Generic discovery cannot initialize the six argument-taking lifecycle
and terminal tests; rerunning them through their documented CLI entry points
passed all six. Go does not support the race detector on Windows ARM64; the native run did not
claim race coverage. Standards and specification reviews reported no findings.

## Environment and limits

After an approved restart of the unresponsive VM, checks that temporarily modify
persistent user PATH ran in UTM disposable mode. Their existing CI-only guard was
enabled only inside that disposable snapshot. Portable Node 22.23.3 and
PowerShell 7.6.6 ARM64 archives were verified against their official SHA256 sums
and added only to the runner process's PATH. The runner used PowerShell's
process-scoped execution policy for local test scripts, matching the CI shell
invocation; no account or machine execution policy was changed.

Initial prerequisite and launch-environment failures are retained in host logs
under `/tmp/tofa56-windows`. These synthetic checks do not replace normal-account,
live-service release qualification.
