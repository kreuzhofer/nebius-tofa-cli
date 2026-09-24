# Experimental Codex desktop launch

Source-build feature for [#32](https://github.com/kreuzhofer/nebius-tofa-cli/issues/32);
not included in v0.1.0-rc.2.

```sh
go build -o tofa ./cmd/tofa
./tofa launch codex-desktop --model moonshotai/Kimi-K3 --allow-unverified
```

Use an existing `tofa auth login`. The model must be explicitly selected and
available in the saved project's catalog. `--project-id ID` overrides that project.
Availability and bundled provider metadata do not certify a supported combination;
the unverified gate remains mandatory.

The gate accepts macOS **26.6.2 arm64**, ChatGPT **26.917.71314 (10954)**,
bundle ID `com.openai.codex`, and bundled engine **0.155.0-alpha.16.4**. The target is
the application's **Codex mode**. The target requires `/bin/zsh` as the account's login shell;
other shells need separate qualification and should use `launch codex`.
Discovery checks `/Applications` then `~/Applications`, with ChatGPT/Codex bundle
names. For another location, pass `--app-bundle '/path/to/ChatGPT.app'`; its identity
and versions still must match. An incompatible client fails with an actionable
error. Windows, Intel Macs, and other app/engine/OS versions require qualification.

## Ownership and routing

Quit Codex before launching through tofa. The launcher uses the ordinary desktop
profile at `~/Library/Application Support/Codex` and the ordinary engine history
location resolved through the account's interactive login shell (`CODEX_HOME`,
or `~/.codex`). It keeps the existing credential store and account/onboarding
files in place. It does not copy credentials, import history, create another
conversation store, or change the current workspace.

An existing desktop causes a refusal with manual recovery instructions. Launcher
attempts serialize using a private lease. The qualified native shell also owns a
profile singleton; the launcher verifies that its newly spawned PID owns it and
receives an authenticated engine-wrapper acknowledgement before publishing live
routing or adding provider metadata. Missing or changed ownership cancels only
that launch. A process scan also refuses live desktops whose native lock is
missing. Native stale-lock recovery is allowed only when the local recorded PID
is demonstrably absent; the launcher never removes native lock/socket/cookie files.

Opening Codex normally while tofa is running reuses the existing app on the tested
client; it does not switch its mode. Quit the app and reopen it normally to return
to ordinary mode. No startup deep link is sent to a possible incumbent.
Unclaimed contenders have a three-second deadline, before the native hung-owner
notification timeout. See the exact-build [ownership evidence](research/desktop-profile-ownership.md)
for the observed collision behavior and limits.

Conflicting inherited Codex/OpenAI/Electron routing variables are removed.
An inherited, nonempty `CODEX_CLI_PATH` must already identify this launcher's
verified bridge; a user-managed executable override is rejected before launch.

The pinned desktop normally reloads an interactive login-shell environment after
startup. A private, per-launch `ZDOTDIR/.zshenv` handles only that exact environment
query, returning the launch-time variables before ordinary startup files can
replace the adapter credential or engine selection. The launcher announces this
behavior. Normal coding-shell commands restore the original `ZDOTDIR` (or its
unset state) and source the original `.zshenv`; normal startup then continues.
User shell files are never edited. This uses zsh's documented
[startup-file ordering](https://zsh.sourceforge.io/Doc/Release/Files.html), with
the environment query pinned to the inspected desktop source.

The bundled engine resolves ordinary configuration and enforced policy before
launch. Incompatible managed requirements and user-owned `nebius-tofa` entries
cause explicit refusal. After native ownership is established, effective live
routing is checked and the engine's version-checked `config/batchWrite` API adds
only the inactive provider entry. A competing edit is an error; cleanup never
restores a whole-file snapshot. Unrelated configuration and edits during the
launch remain intact. The original HOME remains available for native policy
discovery. The checks use the engine's public `config/read`,
`configRequirements/read`, and `config/batchWrite` APIs.

The durable `model_providers.nebius-tofa` entry contains a display name, Responses
wire format, loopback port zero and the deliberately absent
`TOFA_DESKTOP_INACTIVE` credential variable. It contains no live adapter address
or credential. Ordinary mode can hydrate recorded Token Factory history; sending
fails with guidance to relaunch through tofa using
`--model moonshotai/Kimi-K3 --allow-unverified`. The ordinary bridge removes both
stale launch and inactive credential variables. Even a directly invoked engine
with an artificially populated inactive variable cannot reach either inference
provider, although its port-zero connection failure may keep retrying until
interrupted. Choosing GPT does not migrate a conversation's provider.

### Native models and conversation routing

Each launch asks the qualified bundled engine for `debug models` **before**
applying Token Factory overrides, in the target engine home and workspace. The
launcher preserves complete native descriptors, including account-dependent
availability, reasoning choices, instructions and unknown fields. It appends only
`moonshotai/Kimi-K3`, displayed as `Kimi-K3 (Token Factory)`; it neither reconstructs
descriptors from the
lossy `model/list` picker response nor ships a frozen native snapshot. Empty,
malformed, duplicate or conflicting model identities cancel the launch.

The qualified engine's export uses `OnlineIfUncached`: eligible signed-in accounts
use a cache with a **five-minute TTL**, scoped to client version and provider/auth
identity; a miss triggers native catalog discovery. Synthetic tests verify fresh
cache reuse and refresh after expiry, an identity mismatch or a version mismatch.
Unauthenticated and API-key exports use the engine's normal bundled metadata; this
version's debug command does not enable API-key remote discovery. Native picker
filtering still belongs to the engine and its current account.
[Export implementation](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/cli/src/main.rs),
[cache contract](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/models-manager/src/cache.rs).

A synthetic authenticated HTTP 401 demonstrated that `debug models` can return
bundled metadata with exit status zero and no stderr. The launcher therefore also
uses the engine's `login status` boundary. For a ChatGPT account, the export must
match a fresh, versioned native cache; missing, stale or inconsistent evidence
causes an explicit freshness error. A fresh cache belonging to a different account
can happen to match the engine's bundled fallback. Because the export API provides
no refresh-success attestation, signed-in exports identical to `debug models
--bundled` are also refused, even when an account genuinely receives that full
catalog. This conservative compatibility limit is reported explicitly.
The comparison accounts for the export-only
legacy `base_instructions` projection; the merged descriptor itself is unchanged.
Diagnostic-bearing exports and unrecognized login states also fail explicitly.
The launcher does not print native diagnostics, inspect credentials, or copy them.
A signed-in static custom catalog without matching fresh cache evidence is
currently refused; it must be qualified before shared-profile integration.

The resulting override is a **snapshot for this launch**. The engine's static
catalog manager does not hot-reload it or perform remote refresh while it is
active. Relaunch after account, entitlement or catalog changes; this limitation
is printed at startup. A later launch resolves native metadata again. Existing
account files and native provider settings remain the engine's responsibility.
[Static catalog selection](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/model-provider/src/provider.rs).

Cold resume with null model/provider retains the recorded identity. A native Astra
thread continues through its native provider during a tofa launch; a new default
thread identifies `nebius-tofa` / `moonshotai/Kimi-K3`. The picker changes a model,
not its provider: selecting Astra inside a Token Factory thread fails explicitly
at the adapter, without native migration or Kimi substitution. Selecting Kimi again
allows the same thread to continue while the launcher is active.

The merged catalog also exposes native auxiliary choices such as
`codex-auto-review`. They remain unsupported on the Token Factory route, with an
HTTP error and terminal notice. The recognized Kimi title and non-strict review
adaptations, schema/tool restrictions, and compaction rejection are unchanged.
Catalog visibility does not expand supported Token Factory models.

This implements [#46](https://github.com/kreuzhofer/nebius-tofa-cli/issues/46) on
the isolated target. It does not connect ordinary history/account state yet.
[#50](https://github.com/kreuzhofer/nebius-tofa-cli/issues/50) must qualify the
combined production workflow with authorized live-account access: compare native
picker choices and full descriptors with ordinary mode, observe entitlement and
catalog refresh on relaunch, continue a real native conversation during a tofa
launch, and verify onboarding/account continuity and same-thread picker recovery
in the UI. Synthetic account/cache/HTTP checks are not live-account evidence.

### Durable engine bridge

The desktop receives a stable `CODEX_CLI_PATH` under
`<tofa-config>/desktop-bridge-v2/<SHA-256-of-absolute-bundled-engine-path>/tofa-desktop-engine`.
The bridge is a private executable copy of the launcher, with a sibling
`owner.json` recording contract version 2, the absolute bundled engine path, and
the bridge executable's SHA-256. The record contains no credential or live route.
The executable dispatches to bridge mode by its fixed filename; it does not
depend on the original source-build executable remaining in place.

Normal launch cleanup retains this directory. Saved tool references therefore
remain executable. With no `TOFA_DESKTOP_CONTEXT`, the bridge executes the recorded
bundled engine with the caller's ordinary settings, arguments, and exit status.
It removes `TOFA_API_KEY` and `TOFA_DESKTOP_INACTIVE` from that ordinary invocation. Ordinary account
credentials and `HOME` are preserved.

During a launch, `TOFA_DESKTOP_CONTEXT` names the current loopback adapter and
`TOFA_API_KEY` authenticates the bridge's `GET /desktop-launch` request. The route
exists only in launcher memory, binds the bridge, engine, and ordinary
`CODEX_HOME`, and is served with `Cache-Control: no-store`. No on-disk capability
manifest is needed. Empty, malformed, unauthenticated, mismatched, or expired
context is an explicit error with relaunch guidance, even for `--version`;
it never selects ordinary mode. The client permits only IPv4 loopback HTTP,
disables proxy discovery and redirects, and bounds the request time and size.

For `app-server`, the bridge appends live routing overrides at the subcommand's
argument boundary, after desktop/plugin overrides and before any `--` separator.
Other arguments retain their order and values. Startup preflight asks the actual
engine for effective routing and managed policy while the bridge waits for
ownership qualification. Inference is refused until that qualification completes.
The real Token Factory key stays in the launcher. Only the temporary local bearer
reaches the owned desktop, and neither it nor the context is an argument or a
durable bridge setting. Each launch creates a new adapter and bearer.

Initial installation claims a new private directory. Existing records and
executables must match before reuse; missing, edited, or symlinked ownership is
an explicit conflict, never permission to overwrite another executable. A setup
write failure removes only the directory created by that attempt. A fully
installed bridge remains usable even if subsequent desktop setup fails.

The later installed-lifecycle ticket owns upgrade and uninstall integration.
Version 2 adds the startup ownership acknowledgement and installs alongside
version 1. Existing version-1 directories and saved references are left intact;
new launches use version 2. A verified version-2 bridge is reused rather than
replaced on each source launch. That ticket must preserve saved executable paths and
the matching ownership record, define atomic upgrade/recovery (including an
interrupted initial installation), and detach saved references before removal.
Do not delete this directory as ordinary session cleanup. Moving/removing the
bundled application causes an explicit execution error; a launch from a new
bundle path gets a distinct owned bridge. No ordinary profile migration or
installer/uninstaller changes are included in this slice.

The real Token Factory key stays inside the launcher. The owned client gets only
a random per-launch bearer token for an authenticated IPv4 loopback adapter.
Only the selected model and `POST /responses` are accepted. Streaming and existing
history repair are retained. No auxiliary model is silently remapped. Rejected
auxiliary routes/models, schema failures and upstream HTTP errors are reported in
the terminal without request bodies or credentials.

Keep the terminal running. Closing the owned app, Ctrl-C/SIGTERM, adapter failure,
or loss of the owned app-server ends the launch and closes its listener. A stuck
owned process group receives a forced stop after two seconds. The startup deadline
for observing the bundled app-server is 15 seconds. Only that new process group
is signaled; ordinary desktop processes are not targeted.

Ordinary history, Electron state and workspace files remain in their original
locations after exit. Only private preflight/control files under
`<tofa-config>/desktop-launches/launch-*` and the temporary model catalog are
removed. The durable bridge and inactive provider remain; the old listener and
bearer become unusable. Prior isolated `desktop-sessions` are left untouched and
are not automatically imported.

The [#34 contract check](research/desktop-shared-history.md) established the
accepted limitation: Token Factory access lasts only while the launcher runs.
Shared-history engine and launcher tests pass; real-account UI acceptance of the
combined production path remains tracked in #50. Abrupt launcher death and
installer/uninstaller integration remain separate #48/#49 work.

## Limitations and verification

Automatic thread titles work for the captured Kimi desktop contract through a
narrow schema-to-instructions adaptation; the desktop still validates the generated
title and description. The launcher announces this adaptation. Different title
schemas or tool inventories fail explicitly; read-only app title lookups that add
tools remain unqualified. See the [#33 contract and live evidence](research/desktop-title-generation.md).
The recognized non-strict automatic-review workaround remains unchanged. Native
auxiliary model IDs, compaction endpoints, web search, account-backed services, and
Chat/Work/voice are not qualified. No `--direct`, Guardian evaluation model, profile,
or arbitrary desktop argument passthrough is exposed.

Offline executable/HTTP tests cover shared-state preservation, credential separation, selected
model streaming, auxiliary rejection, managed/effective routing conflicts, startup
and exit errors, engine loss, cancellation, adapter failure, and owned-worker cleanup.
They reproduce the desktop's zsh environment query, including conflicting shell
exports and normal coding-shell startup with default/custom `ZDOTDIR`.
The desktop executable tests run on the exact gated macOS/architecture combination;
other hosts skip them. Portable launcher/adapter regressions still run normally.
To additionally exercise the actual bundled engine against dummy offline settings:

```sh
TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
  go test ./internal/tofa -run '^TestDesktop' -count=1
```

This optional test starts the real engine but a fixture desktop executable; it does
not perform paid inference. Its bridge regression completes a synthetic turn
through the authenticated request adapter and checks the upstream model and
credential, including conflicting app-server routing and unrelated plugin
overrides. This proves fixture routing, beyond merely avoiding paid inference.
The earlier human-operated streaming/tool/continuation
proof and its limitations remain in the [feasibility evidence](research/codex-desktop-feasibility.md).

An additional opt-in test starts the installed Electron app with dummy credentials,
synthetic conflicting shell exports and fresh state, checks the owned bundled
engine's temporary credential against a local fixture, then stops the owned app:

```sh
TOFA_TEST_DESKTOP_APP=/Applications/ChatGPT.app \
  go test ./internal/tofa -run '^TestDesktopInstalledAppShellIsolation$' -count=1 -v
```

This passed on the updated pinned combination on 2026-09-24. It uses disposable
HOME, Electron and engine state, a test-only mock keychain and disabled updater.
It uses no paid inference;
it verifies that shell-exported credentials and an engine override do not reach
the actual desktop engine. It does not claim another live-model coding session.

### Live source-build check, 2026-09-23

The user completed three turns in one isolated conversation using this launcher:
a real `exec_command` ran `printf tofa-desktop-32-ok`, a second turn recalled that
marker, and a third requested a longer prose response. The user reported that the
longer answer appeared to stream; streaming was unclear on the short answers.
The owned session record independently contains the tool call/result and three
completed turns. No live wire-event counts were recorded. Cumulative session usage
was 41,558 input tokens (including 30,976 cached) and 342 output tokens.
[Sanitized evidence](research/evidence/codex-desktop-launch-2026-09-23.json).

The first live attempt exposed a command-shape bug: the desktop places `-c`
options before the `app-server` subcommand. The launcher stopped its own instance
after failing to recognize it. An executable regression reproduced the issue;
the corrected detector accepted the live app and remained attached for all three
turns. Automatic title generation failed explicitly, as announced.

After interruption, all 12 observed owned processes exited, the adapter port
closed, and temporary routing/catalog files were removed. Conversation and
workspace files remained. The ordinary app/engine PIDs stayed alive; ordinary
`config.toml` content and metadata and `auth.json` stat metadata were unchanged.
Ordinary credential contents were not read. This preservation check does not
claim a separate functional test of ordinary account-backed inference.

Final validation passed: `go test -race ./...` (including the optional installed-engine
check), `go vet ./...`, Linux/Windows amd64 cross-compilation, and the two native
terminal smoke tests. Independent standards and spec reviews reported no findings.
The race suite found an output-writer race after the live check; a per-launch mutex
fixed it, and the full suite passed on rerun. The evidence pins the live binary
before that output-only correction.

### Bridge validation, 2026-09-24 (#45)

Before the fix, the saved-reference regression failed because the desktop had no
durable launcher-owned executable to save. The bundled-engine regression then
reproduced displaced live routing when app-server overrides selected `openai`.
After the fix, a saved bridge executes after cleanup; ordinary settings and exit
status survive; expired/malformed/mismatched contexts fail; relaunch preserves the
bridge while rotating credentials and endpoints. Ownership-conflict and actual
file-size-limit write-failure tests cover safe setup. The fixture's retained tool
settings, history, and bridge artifacts are checked for live capability leaks.

The local app auto-updated to engine `0.155.0-alpha.16.3` during this work. Engine
tests used a retained `0.155.0-alpha.9.2` executable from the earlier isolated
prototype's plugin directory. The production version gate remains unchanged.
This evidence qualifies the bridge at the executable/HTTP and bundled-engine
boundaries; it does not qualify the updated Electron app or ordinary-profile
history. All credentials and inference responses in these checks are synthetic.

Final checks passed: the complete `go test -race ./...` suite with the qualified
engine enabled, `go vet ./...`, and Linux/Windows amd64 builds. Separate standards
and spec reviews have no remaining findings after correcting argument-value
detection and removing duplicate test setup.

### Native catalog validation, 2026-09-24 (#46)

Regressions first reproduced the Token Factory-only catalog at the launcher and
actual-engine picker boundaries, then reproduced authenticated discovery silently
falling back after HTTP 401. Review found and reproduced the additional case of
a foreign-account cache containing bundled-equivalent descriptors. The corrected
launcher rejects both failures, with an explicit conservative refusal when even a
successful account catalog is indistinguishable from the bundled fallback.

Checks use synthetic credentials, temporary profiles and separate native and
Token Factory HTTP fixtures. The retained engine is **0.155.0-alpha.9.2**, SHA-256
`9280c0754e8f1f6b72f495d30c8c82a006dbc4995bf0492916fa0901f6bfd1f9`, on macOS
**26.6.2 arm64**, compiled with Go **1.27.1**. The installed app has since updated;
these checks do not change the production gate or qualify that newer app.

Native cold resume, new Token Factory identity, explicit Astra failure, same-thread
Kimi recovery, native auxiliary rejection, descriptor preservation and native
cache/account scenarios passed. Separate Standards and Spec reviews reported zero
remaining findings after the freshness correction. No installed Electron UI,
ordinary live account or paid inference was exercised for this ticket.

Final validation passed: `go test -race ./...` with `TOFA_TEST_DESKTOP_ENGINE`
pointing to that retained executable, `go vet ./...`, and Linux/Windows amd64
builds. The opt-in installed-Electron check was not enabled.

### Shared profile validation, 2026-09-24 (#47)

The missing-provider regression reproduced ordinary resume failing before inactive
metadata was added. The production launcher now completes ordinary → tofa →
ordinary → tofa and repeated relaunches through the actual alpha.16.4 engine.
Public APIs verify two unique identities, ordered messages and an executed
`printf` command/result, titles, workspace, native routing, inactive send failure,
and same-thread recovery with a fresh launch. The full tool-history test uses the
updated client's paginated history contract; legacy hydration omits those tool
items. Synthetic native and Token Factory HTTP fixtures verify which route is used.

Launcher regressions cover existing owners, missing native locks, competing
launchers, ownership loss or owner exit before integration, version-conflicting config edits,
and preservation of edits during a launch. An actual installed-desktop startup
check verifies the native ownership/bridge handshake and working local credential.
Five separate native diagnostics cover both launch directions, simultaneous
starts and native stale-state recovery. Run them separately from launcher tests:
ordinary diagnostic launches intentionally trigger the launcher's incumbent
refusal. The evidence also retains one concurrent native-quit timeout. [Build provenance and sanitized evidence](research/evidence/desktop-profile-ownership-2026-09-24.json).

These checks use synthetic credentials and disposable profiles. They do not claim
real-account onboarding continuity, an observed UI history walkthrough or paid
Token Factory inference on this combined implementation. Historical #32/#45/#46
results above retain their original versions and scope.

Final checks passed: `go test -race ./...` with both installed-engine and
installed-desktop opt-ins, `go vet ./...`, Linux/Windows amd64 builds, three
installed-engine history diagnostics and five installed-native ownership
diagnostics. The final Go and native suites ran sequentially. Independent
Standards and Spec reviews have no remaining confirmed findings.
