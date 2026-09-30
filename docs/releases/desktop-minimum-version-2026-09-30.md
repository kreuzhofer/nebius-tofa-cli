# Desktop minimum-version adaptation, 2026-09-30

Issue #35 now admits compatible newer desktop releases instead of requiring one
exact app/build/engine version. The minimums are macOS 26.6.2 arm64, ChatGPT desktop
26.917.71314 (`com.openai.codex`) and engine 0.155.0-alpha.16.4. Exact build numbers
and hashes remain qualification evidence. This checkpoint does not publish a
prerelease or establish the remaining visible desktop/live-provider checks.

The implementation supports the native packaged macOS engine, validates catalog
cache versions against the selected engine, and repairs ordinary saved references
when an app update moves the engine within the same bundle. Installer upgrades
validate both recorded layouts. Incomplete packages, below-minimum versions and
incompatible ownership/configuration still fail explicitly.

The [primary-source investigation](../research/desktop-minimum-version-2026-09-30.md)
identifies ChatGPT 26.928.21956 (12404), native engine 0.159.2 and its exact hashes.
The adjacent `codex-cli/bin/codex` is not the native macOS engine; tests select
`codex-cli/CodexCLI.app/Contents/MacOS/codex`.

## Validation checkpoint

On macOS 26.6.2 arm64 with Go 1.27.1:

- Minimum-version, packaged-layout, incomplete-package and saved-reference tests
  reproduced failures before implementation and passed after their fixes.
- Actual installer/uninstaller regression passed after an engine relocation.
- The new engine's authenticated catalog suite passed. The minimum engine's
  authenticated catalog suite also passed, including same-version foreign-cache
  ambiguity and refresh behavior.
- The initial complete race run recorded 447 passing test/subtest entries, three
  opt-in skips, and one failing leaf test (plus its enclosing test). Its assertion
  expected four Guardian HTTP 503 attempts; engine 0.159.2 made twelve. Execution
  remained denied. Retry count is engine-owned, so the corrected test requires
  bounded failure and no execution. Its targeted race rerun passed alongside the
  corrected catalog regressions. This initial run is retained as a failure.
- Go vet, all six platform builds, six packaging tests, four native distribution
  lifecycle tests and two terminal tests passed. Cross-builds are not native
  Linux/Windows qualification.
- An initial generic Python test-discovery invocation did not provide the
  lifecycle/terminal scripts' required arguments. Their proper entrypoints passed
  on rerun. The packaging check also caught the newly added x/mod dependency's
  missing notice in an earlier source snapshot; the notice was added and all six
  packaging checks passed on the corrected snapshot.
- Standards review found a stale foreign-cache fixture version; the fixture now
  matches the selected engine and asserts the ambiguity refusal. Final Standards
  and Spec reviews have zero remaining findings.

The installed-Electron startup attempt refused an already-running ordinary app,
without changing it. Final full-suite confirmation, installed-Electron ownership,
real-account UI and the immutable candidate lifecycle remain qualification work.
Earlier rc6/rc7 evidence and bytes are retained; no existing candidate is retagged.

## rc8 real-profile startup finding

The corrected minimum-version full race suite passed 449 test/subtest entries
with three opt-in skips. CI run 36728281247 passed artifacts and all three native
platform jobs. rc8 was installed through its actual installer; repeat install,
uninstall without purge, reinstall, saved-login reuse, fresh-shell PATH, settings
and existing-history preservation passed.

The actual desktop then closed itself before any chat was sent, reproduced in
three separately retained attempts. The temporary native network-requirements
helper closed normally before the main conversation engine started. The old
monitor treated its exit as terminal and kept that decision even when the main
engine was alive. The process capture distinguishes this observer error from an
engine crash. [Sanitized rc8 evidence](evidence/desktop-rc8-startup-2026-09-30.json).

The executable regression reproduced the failure. The bridge now acknowledges
only the successful named main initialization exchange, observed on both client
versions, and records that engine PID for the launch. The authenticated adapter
rejects replacement registrations; the monitor still validates the owned process
group, executable and app-server invocation, and still shuts down on established
main-engine loss. Startup remains bounded when a client changes this contract.

Targeted race checks passed for the startup handoff, immutable registration,
engine loss, graceful exit, cancellation and sustained installed-desktop startup.
The native ownership harness initially assumed synchronous stdout logging and
that every observed engine remained alive; both assumptions failed with startup
helpers. It now waits for the main initialization log, reads only logs associated
with its own child PID in the synthetic home, and excludes exited helpers. All
five native ownership checks passed. These observer failures remain recorded.

Both reviews report zero remaining findings for the production fix. Full-suite
confirmation passed 452 test/subtest entries with two CLI opt-in skips in 692.788
seconds; both installed-desktop opt-ins ran. rc8 remains a failed, unpublished candidate;
its frozen bytes will not be replaced. A fresh candidate is required for the
remaining ordinary-profile UI and live-provider qualification.

## rc9 startup recovery and history validation

rc9 was frozen from `2db08bcdff3828704e96a5c70207d2bea35bb650`. Its macOS
ARM64 binary SHA-256 is
`ee2b005cabcd1c40794fd94db6d3a59673b9fbba193a310850c90104b4c658b3`.
CI run 36731717915 passed. Six packaging, four native distribution lifecycle and
two terminal checks passed. The actual installer upgrade, repeat install,
uninstall without purge, reinstall, saved-login reuse and fresh-shell PATH
checks passed with settings, credentials and existing history preserved.

The original ordinary-profile startup reproduction now stays open. The user
confirmed existing history and the eligible picker with Experimental labels.
They completed a DeepSeek tool call, switched that conversation to GLM 5.3 Flash,
and confirmed continued history. Read-only metadata independently records the
two models, one tool call/result and two completed turns. Changing the desktop
default to GLM 5.3 left the concurrent CLI model and reasoning unchanged.

Separate requests in that launch returned HTTP 422: Token Factory required an
`id` on an assistant message at `body.input[4]`. The rejected item already had
`status: completed` and output-text annotations; it omitted the ID. Only the
validation fields and item shape were retained, without conversation text or
credentials. These failures are separate from the passing user tool check.

A public adapter regression reproduced that rejection before the repair. The
adapter now supplies a missing assistant-message ID, deterministically for
identical retries and appended turns, without replacing existing IDs or changing
stored history. Repeated identical messages receive distinct IDs by input
position. This does not promise stable IDs across arbitrary history rewrites.
Adapter race regressions and vet passed; Standards and Spec reviews have zero
findings. An existing Guardian preservation fixture needed an explicit original
ID to retain its assertion that unrelated fields remain unchanged; its initial
failure and passing rerun are recorded separately.

The full suite for this additional repair is running. Fresh-candidate live
qualification remains necessary; rc9 is not published or claimed complete.
