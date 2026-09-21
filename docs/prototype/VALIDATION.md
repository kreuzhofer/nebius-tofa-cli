# Prototype evidence — 2026-09-21

Status: the accepted per-launch request adapter is implemented and passes local
checks. The maintainer reports a successful live game build and follow-up change.
The maintainer also confirms that ordinary Codex selects its original model after
testing. Native Linux/Windows adapter checks and the complete compatibility record
remain; the Wayfinder prototype decision is open. No model is certified.

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

## Still needed before compatibility/support claims

1. Maintainer feedback on the CLI and code walkthrough.
2. Native credential read/write/delete on each OS, including headless/locked/denied
   stores. Windows file ACL enforcement has CI coverage; actual credential-store
   checks require an isolated test account
   or explicitly authorized test store; this session did not modify user credentials.
3. Windows user PATH update and asynchronous self-removal; fish execution. The
   Windows offline install/update/uninstall lifecycle now has passing CI coverage.
4. Complete the live compatibility record: the maintainer's successful game-build
   and follow-up report is recorded below. Capture the exact model/client/platform
   combination and explicit streaming evidence before changing support claims.
   Any agent-initiated paid checks still need a separately authorized budget.
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
| Windows/Linux x64 test executables | Cross-compiled; not executed here |

The optional installed-client test selects the Kimi model ID but serves every
response locally. The tool executes only `printf tofa-fixture-tool` in a scratch
directory. These checks use synthetic keys, a fake credential store and scratch
client configuration. They do not call Token Factory or any inference service,
access saved user credentials, or certify Kimi's actual model behavior.

The updated macOS binary is `dist/tofa_v0.0.0-prototype_darwin_arm64`. It announces
the default adapter route. Add `--direct` only for deliberate diagnostic bypass.

### Remaining adapter acceptance checks

- Execute the new suite on native Linux and Windows runners. Six successful
  cross-builds and the old native CI results do not substitute for this.
- The maintainer has completed the live build-and-follow-up exercise and confirmed
  ordinary Codex selects its original model afterward. Capture the exact live
  session version/platform and streaming behavior for the compatibility record.
  No agent-initiated paid inference is authorized.
- Immediate-child cancellation is implemented; Unix sends an interrupt then kills
  after two seconds, Windows kills the immediate child. Descendant containment,
  detached processes and abrupt launcher death need further platform evidence.
  The embedded endpoint ends with the launcher; that does not prove every client
  descendant exits. Windows termination behavior is cross-compiled, not run here.
- Kimi model metadata is supplied from the provider evidence described below.
  Other models require their own capability evidence. The supported-model registry
  remains empty pending a repeatable live compatibility qualification process.

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
Codex 0.155.1 on macOS ARM64. Explicit streaming behavior remains to confirm.

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
repeated local race tests pass. A new native run is required before replacing
the remaining native-validation limitations above.
