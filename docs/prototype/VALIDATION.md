# Prototype evidence — 2026-09-21

Status: the accepted per-launch request adapter passes native checks. The
maintainer reports a successful live game build, follow-up change and ordinary
Codex model restoration. Three automated live sessions now qualify Kimi-K3 with
Codex 0.155.1 on macOS ARM64 for the recorded streaming/tool/continuation checks.
The experimental launch gate remains; no other model/client/platform is qualified.

## Original direct-prototype evidence

Development host: macOS ARM64. Temporary official Go 1.27.1 toolchain, downloaded
with its SHA-256 verified. The user's system Go/PATH was not changed.
Installed client observed through `--version`/`--help`: Codex CLI 0.154.0.

| Check | Result |
| --- | --- |
| `go test -race ./...` | Passed; synthetic vault, loopback catalog and real fake-client process |
| `go vet ./...` | Passed |
| Native compiled CLI `--help` / `--version` | Passed |
| Asterisk feedback on typing/paste, both backspace encodings, plaintext opt-in login/logout | Passed in a pseudo-terminal using synthetic key and temporary HOME; plaintext key never echoed |
| Ctrl-C during key input restores terminal echo | Failed initially; fixed; passed |
| Failed native-store replacement retains prior login | Passed with fake vault |
| Config commit failure rolls back replacement | Passed with simulated filesystem failure |
| Explicit fallback, failed logout, permission/symlink rejection | Passed with scratch files/fake vault |
| Redirect rejection; project/auth catalog request; unverified gate | Passed against loopback HTTP |
| Child key in environment, not argv; parent/config preservation; exit 23 propagation | Passed with an actual fake executable |
| Unix install/rerun, PATH activation/deduplication, checksum failure, uninstall/purge | Passed with local fixtures, paths containing spaces/apostrophes |
| Damaged shell marker preserves unrelated startup text | Failed initially; fixed; passed |
| Shell script syntax | Passed |

These tests intentionally never access native key stores or real API credentials.
A fake vault establishes the launcher's behavior around failure, not OS integration.
No Token Factory request or paid inference was performed.

## Original direct-prototype build and native-execution matrix

| Target | Cross-build | Native execution evidence |
| --- | --- | --- |
| macOS ARM64 | Passed, CGO disabled | Local and CI CLI/PTY/Unix lifecycle tests |
| macOS x64 | Passed, CGO disabled | Not run locally |
| Linux ARM64 | Passed, CGO disabled | Not run locally |
| Linux x64 | Passed, CGO disabled | CI tests/vet, Unix lifecycle and terminal tests passed |
| Windows ARM64 | Passed, CGO disabled | Not run locally |
| Windows x64 | Passed, CGO disabled | CI tests/vet, real filesystem ACL checks, PowerShell parsing and offline installer lifecycle passed |

[Native CI run](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/35593768301)
for implementation/test commit `24aaf3c` passed its macOS ARM64, Linux x64 and
Windows x64 jobs. Runner architectures were read from the actual Go version logs.
That run predates the request adapter and does not establish its native coverage.

The first Windows run exposed an overly strict textual ACL comparison. The fix
checks the protected DACL's actual entry type and owner SID. Windows tests now cover
real private file creation/readback and rejection of an Everyone-accessible key file.
The native OS credential store remains substituted with a fake vault.

The Windows offline lifecycle test substitutes downloads, uses a temporary
LOCALAPPDATA and `-NoModifyPath`, and covers install/rerun, failed checksum retention,
default preservation and purge without touching Credential Manager. Windows user
PATH updates, asynchronous self-removal and fish startup execution remain unverified.
Native CLI self-uninstall also passed locally in a temporary macOS installation.

## Remaining review and broader-support limits

1. Further maintainer review of the CLI and code walkthrough; the recorded
   hands-on feedback covers the coding workflow, follow-up and normal model restoration.
2. Native credential read/write/delete on each OS, including headless/locked/denied
   stores. Windows file ACL enforcement has CI coverage; actual credential-store
   checks require an isolated test account
   or explicitly authorized test store; this session did not modify user credentials.
3. Windows user PATH update and asynchronous self-removal; fish execution. The
   Windows offline install/update/uninstall lifecycle now has passing CI coverage.
4. The exact live compatibility record is complete for Kimi-K3/Codex 0.155.1/macOS
   ARM64; see the automated qualification below. Other combinations need their own evidence.
   The maintainer authorized in-scope live checks without a spending cap on
   2026-09-21, then approved the three-run Kimi harness and use of saved credentials.
5. Ordinary Codex model selection after a real launched session is confirmed by
   the maintainer. File/environment isolation also passes locally with a fake
   client. These checks do not independently inspect every login/provider setting.
6. Before publishing installable releases: dependency license bundle, release assets
   and checksums. Public curl installation is not yet demonstrated because no release
   was published. Production signing/hardening remains outside this prototype.

Model compatibility registry: **empty**. Every catalog entry is marked unverified;
`--allow-unverified` is required. Browser authorization and broader protocol
translation remain deferred. The narrow Responses adapter is now implemented.

## First hands-on compatibility finding

The maintainer reports that Codex with `moonshotai/Kimi-K3` starts and answers,
then receives HTTP 422 on the next turn: replayed assistant history omits
`message.status` and `output_text.annotations`, which Token Factory requires for
that representation. This is a demonstrated continuation failure, not merely a
model-catalog warning. No credentials or paid calls were used by the assistant to
investigate it.

`scripts/repro_codex_history.py` exercises installed Codex 0.154.0 against a local
synthetic Responses server and captures the same omissions. A control run using
`TOFA_REPRO_MODEL=gpt-5.4` removes the metadata warning while preserving the
serialization failure. Both runs finish with the expected diagnostic exit 1 and
`REPRODUCED: message.status, output_text.annotations`. This optional diagnostic is
not part of the passing Go/installer suite and does not run a real model.

See [the source/schema investigation](../research/codex-kimi-followup.md) for pins,
remaining metadata questions and an engineering inquiry. The accepted temporary
request adapter is implemented as described below. The API-key asterisk change is
independent of this compatibility repair.

## Request adapter validation

Validated locally on macOS ARM64 with Go 1.27.1 and installed Codex CLI 0.155.1.
The adapter implementation is on `prototype/direct-launcher`; the older CI run
above tested the direct prototype, not the adapter changes.

| Check | Current result |
| --- | --- |
| Full `go test -race ./...`, including optional installed-client check | Passed |
| `go vet ./...` | Passed |
| Missing history `status` / `annotations` regression | Passed against a local validator returning 422 for missing fields |
| Existing/null values, large integers, unknown fields, tool/reasoning history | Preserved in HTTP-boundary tests |
| Local bearer authentication, fixed upstream/project, credential isolation | Passed; upstream key absent from child arguments/environment |
| Route/method/encoding/body limits | Passed; rejected requests never reach upstream |
| Streaming and client/launch cancellation | Passed; first event arrives before upstream completes; cancellation reaches upstream |
| Redirect rejection, no retry of 422/429/500/503 | Passed with local upstream fixtures |
| Parallel launches | Distinct listener ports and tokens; cross-launch token rejected |
| Child exit, listener/child startup failure, unexpected listener failure | Endpoint cleanup and explicit errors passed |
| Unresponsive direct child | Cancellation and adapter failure force exit within deadline; normal exit 23 preserved |
| Unix SIGINT / SIGTERM | Passed in isolated launcher subprocesses; unresponsive child terminated and adapter closed |
| Installed Codex 0.155.1, synthetic `moonshotai/Kimi-K3` responses | Tool output round trip and resumed conversation passed; history accepted by strict local validator |
| Scratch Codex configuration/login | Config sentinel unchanged; no auth file created |
| Compiled CLI terminal input checks | Passed, including masking and interruption |
| Six OS/CPU binaries, CGO disabled | Cross-build passed |
| Native macOS ARM64, Linux x64 and Windows x64 | Race suite and vet passed in CI; see the pinned run below |

The optional installed-client test selects the Kimi model ID but serves every
response locally. The tool executes only `printf tofa-fixture-tool` in a scratch
directory. These checks use synthetic keys, a fake credential store and scratch
client configuration. They do not call Token Factory or any inference service,
access saved user credentials, or certify Kimi's actual model behavior.

The updated macOS binary is `dist/tofa_v0.0.0-prototype_darwin_arm64`. It announces
the default adapter route. Add `--direct` only for deliberate diagnostic bypass.

### Remaining adapter acceptance checks

- Native race/vet checks pass on macOS ARM64, Linux x64 and Windows x64.
  macOS x64, Linux ARM64 and Windows ARM64 have build evidence only.
- The maintainer has completed the live build-and-follow-up exercise and confirmed
  ordinary Codex selects its original model afterward. The automated qualification
  below adds exact live identifiers and streaming evidence.
  The later live-harness authorization supersedes the earlier budget prerequisite.
- Immediate-child cancellation is implemented; Unix sends an interrupt then kills
  after two seconds, Windows kills the immediate child. Descendant containment,
  detached processes and abrupt launcher death need further platform evidence.
  The embedded endpoint ends with the launcher; that does not prove every client
  descendant exits. Immediate-child termination now also passes native Windows CI.
- Kimi model metadata is supplied from the provider evidence described below.
  Other models require their own capability evidence. The supported-model registry
  remains empty; the opt-in harness now provides repeatable qualification evidence
  without changing the prototype's explicit unverified-model launch gate.

### Maintainer hands-on report

After receiving the rebuilt-launch command for `moonshotai/Kimi-K3`, the maintainer
reported that discovery succeeded while the metadata warning remained. They then
tested in an empty repository using the suggested browser-game exercise: build a
Breakout game, followed by a request for a temporary paddle-widening power-up.
Their result: **"both completed successfully, game works as expected"**.

This is user-reported live evidence for a useful coding task, tool use and a
successful follow-up without the previously reported continuation failure. It is
separate from the automated synthetic-response check above. The recommended flow
used the adapted Kimi route; no fresh session log or exact launch command was
provided, so the live model/client/platform identifiers are not independently
captured. The locally observed client for the preceding automated check was
Codex 0.155.1 on macOS ARM64. Streaming was unconfirmed at this checkpoint; the
later automated qualification below supplies that evidence.

Asked whether launching `codex` normally, without `tofa`, still used the usual
provider and login, the maintainer replied: **"yes, it falls back to its original
model when launched"**. This confirms the ordinary model selection is preserved
after the live test; it describes a separate ordinary Codex launch, not an
automatic route fallback inside `tofa`. No user authentication files were inspected
and no agent-initiated inference was run.

## Kimi metadata and native-CI follow-up

The metadata warning is reproduced independently with a single synthetic response
in Codex 0.155.1. A context override alone leaves the warning intact; an exact-slug
catalog entry removes it. The [pinned research and diagnostic](https://github.com/kreuzhofer/nebius-tofa-cli/tree/03d47a502c09debc36a2072c3aa3a929beb6c38d)
distinguish this client lookup from real model inference and provider capabilities.

The implementation now supplies a per-launch Kimi catalog grounded in Nebius's
public model record: context/max-context 1,024,000 and text/image modalities.
It preserves the pinned default Codex coding instructions and does not advertise
unverified reasoning-effort, summary or verbosity controls. The provider-specific
output ceiling and detailed Responses reasoning contract remain unknown; neither
is invented. The metadata does not itself certify a model/client combination.

Local `go test -race ./...`, `go vet ./...`, the six-artifact build and terminal
checks pass with these changes. The installed Codex 0.155.1 synthetic integration
now explicitly checks that the metadata warning is absent, coding instructions
remain, tool execution succeeds and a follow-up completes. HTTP-boundary checks
also cover catalog cleanup on success/failure, explicit creation failure and
leaving unknown model IDs alone.

The [first adapter CI run](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/35605720621)
passed Windows but caught a shutdown edge case on Linux/macOS: an idle TCP
connection could outlast graceful shutdown and incorrectly report a failed launch.
A deterministic idle-connection regression reproduced the failure. The fix cancels
upstream work and closes all launch-owned connections when the client exits;
repeated local race tests pass.

[Native CI for the adapter and metadata implementation](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/35607849384)
at commit `d60376888c9508faaee0625ee40580f816785c09` passes the race suite and vet
on macOS ARM64, Linux x64 and Windows x64. Architectures were verified in each
job's Go version log. Unix installer and terminal checks pass on macOS/Linux;
PowerShell parsing and offline installer lifecycle checks pass on Windows.
The six-target artifact build and upload also pass, including the vendored Codex
prompt's license and notice files in the artifact bundle.
These jobs do not install Codex: the optional real-client synthetic integration
was run locally on macOS ARM64 with Codex 0.155.1, not across the CI matrix.
No CI check performs real inference or accesses the native credential stores.

## Automated live qualification

The maintainer approved three independent Kimi runs, the real launcher command as
the test boundary, streaming/tool/file/continuation checks, and read-only use of
saved launcher credentials. Spending was already unrestricted for these checks.

The [final sanitized report](evidence/kimi-codex-0.155.1-macos-arm64.json) passes
**3/3 runs and 6/6 turns** on macOS ARM64 (Darwin 25.6.0), Codex CLI **0.155.1**,
model **`moonshotai/Kimi-K3`**, using the default adapted route plus a test-only
loopback SSE observer. All 22 observed Responses requests completed with HTTP 200.
There were 16 successful shell-tool completions, six independently checked JSON
results, and three continuations of the exact original sessions. Every turn
exposed multiple nonempty text deltas before completion. No metadata warning occurred.

Normal Codex config/auth and launcher config/file-credential states were unchanged.
Scratch config remained unchanged and no scratch client auth file was created.
The harness performs no login/logout or credential-store writes. It stores counts,
statuses, timing and binary/script hashes, not credentials or conversation bodies.
See [the harness guide](LIVE-COMPATIBILITY.md) for commands and limits.

Launcher code: `e611742ed14eb83f78545f4e99b62cf5edd14aac`, built with Go 1.27.1 as
`tofa v0.0.0-prototype-live-e611742`. Harness code matches `5490f0b`; the report
contains exact launcher, client and harness SHA-256 values. A later observer
startup fix removes a needless reverse-DNS lookup when binding numeric loopback;
an offline regression injects unavailable DNS to cover it. The report pins the
pre-startup-fix harness; the launcher, forwarding and qualification logic are unchanged.

### Earlier rejected qualification attempts

The [initial report](evidence/kimi-codex-0.155.1-macos-arm64-initial.json) remains
available. Two observer assumptions caused false failures: Codex persists workspace
trust in a fresh scratch config, and clients may close after `response.completed`.
Both were reproduced offline before fixing the harness. One response in that
attempt also terminated before completion; its cause was not established and the
later successes do not erase it or establish a reliability SLA.

The [second report](evidence/kimi-codex-0.155.1-macos-arm64-auth-change.json) passed
all three functional runs but correctly failed the overall preservation check.
The ordinary Codex auth file was modified during that interval; the report cannot
attribute the writer. No restoration or credential edits were attempted. Reporting
now identifies each changed file without disclosing its contents/hashes, and
version probes also use scratch settings. The final full run passes preservation.

Local checks pass: Go race tests, Go vet and Python
syntax checks. Live checks apply only to this exact combination and small task;
they do not establish general coding quality, images, long context, reasoning
controls, other platforms, native credential write/delete behavior or descendants'
lifecycle. The earlier maintainer game review provides complementary live evidence.

### Native harness regression coverage

All **12 offline harness tests** pass locally and on native macOS ARM64/Linux x64
in [the final native run](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/35617480179)
at `cf6dade9969e29f57c2951cb12646dea59395577`. The Go race/vet and platform installer
checks also pass on macOS ARM64, Linux x64 and Windows x64. The live harness itself
is Unix-only; Windows CI does not run it. All six OS/CPU artifacts build and upload
successfully in the same run. Cross-builds do not establish native execution on
the other three CPU/OS combinations.

Earlier macOS harness CI failures were traced to `HTTPServer.server_bind` calling
`socket.getfqdn` for the numeric loopback address. A captured stack established the
blocking lookup; proxy isolation did not fix it. The observer now binds directly
without reverse DNS, retaining HTTP server address metadata. A regression injecting
unavailable DNS fails before this change and passes afterward. Temporary stack
instrumentation has been removed. Fixture requests also explicitly bypass ambient
proxy discovery to keep synthetic traffic local.

## Native Windows candidate installation lifecycle

[CI run 35640269120](https://github.com/kreuzhofer/nebius-tofa-cli/actions/runs/35640269120)
at `7cf7893` passes the actual-candidate lifecycle on Windows amd64 with Go 1.27.1.
The matching bundled PowerShell installer installs `v0.0.0-ci`, and a fresh process
reconstructs PATH from the machine/user stores and verifies resolution, version
and help. Repeated installation adds no duplicate entry. Real CLI uninstall starts
its asynchronous helper; the test waits with a deadline, checks completion and
verifies files and PATH before accepting success. Ordinary uninstall preserves
synthetic settings/credentials, reinstall retains them, and explicit purge removes
owned state while keeping unrelated files and synthetic credentials.

The same run passes corrupt-upgrade rejection, blocked-purge failure and retry,
and recovery-helper cancellation/timeout checks at the public `-WaitPid` boundary.
The harness confirms test-file cleanup and restores user PATH without changing
execution policy. All inference and native credential-store operations are excluded.
Native race/vet checks, macOS/Linux lifecycle checks and six-target distribution
checks also pass. Windows ARM64 still has build evidence only.

The first native run rejected Git Bash's binary checksum markers. A local
regression reproduced the incompatible manifest before the builder was changed to
hash binary bytes explicitly and normalize marker formatting. A separate
test-first fix preserves Windows installation ownership when unrelated files remain,
allowing ordinary uninstall followed by reinstall. Purge relinquishes ownership.
