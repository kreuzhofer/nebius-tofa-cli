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

The initial gate accepts macOS **26.6.2 arm64**, ChatGPT **26.915.31945 (9922)**,
bundle ID `com.openai.codex`, and bundled engine **0.155.0-alpha.9.2**. The target is
the application's **Codex mode**, selected with `codex://threads/new?mode=codex`.
The isolated target also requires `/bin/zsh` as the account's login shell;
other shells need separate qualification and should use `launch codex`.
Discovery checks `/Applications` then `~/Applications`, with ChatGPT/Codex bundle
names. For another location, pass `--app-bundle '/path/to/ChatGPT.app'`; its identity
and versions still must match. An incompatible client fails with an actionable
error. Windows, Intel Macs, and other app/engine/OS versions require qualification.

## Ownership and routing

The launcher executes the app binary directly in its own process group. It supplies
separate `CODEX_HOME`, Electron user data, an empty workspace, and a verified
one-model catalog. It does not use `open`, restart the ordinary app, copy account
credentials, write ordinary configuration, or adopt an existing app-server.
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

The bundled engine resolves configuration in that isolated home/workspace before
launch. The launcher checks the effective model, provider, endpoint, authentication
channel, model catalog, and database/log locations. Unexpected provider headers or
authentication settings fail closed. Managed routing, model, residency, credential
storage, or state-location requirements are currently rejected explicitly;
they are never rewritten. Other enforced policy remains the engine's responsibility.
The original HOME is retained for native policy discovery. The preflight uses
the engine's [`config/read` and `configRequirements/read` APIs](https://learn.chatgpt.com/docs/app-server);
configuration and requirements are [separate layers](https://learn.chatgpt.com/docs/config-file/config-basic).

### Durable engine bridge

The desktop receives a stable `CODEX_CLI_PATH` under
`<tofa-config>/desktop-bridge-v1/<SHA-256-of-absolute-bundled-engine-path>/tofa-desktop-engine`.
The bridge is a private executable copy of the launcher, with a sibling
`owner.json` recording contract version 1, the absolute bundled engine path, and
the bridge executable's SHA-256. The record contains no credential or live route.
The executable dispatches to bridge mode by its fixed filename; it does not
depend on the original source-build executable remaining in place.

Normal launch cleanup retains this directory. Saved tool references therefore
remain executable. With no `TOFA_DESKTOP_CONTEXT`, the bridge executes the recorded
bundled engine with the caller's ordinary settings, arguments, and exit status.
It removes a stale `TOFA_API_KEY` from that ordinary invocation. Ordinary account
credentials and `HOME` are preserved.

During a launch, `TOFA_DESKTOP_CONTEXT` names the current loopback adapter and
`TOFA_API_KEY` authenticates the bridge's `GET /desktop-launch` request. The route
exists only in launcher memory, binds the bridge, engine, and isolated
`CODEX_HOME`, and is served with `Cache-Control: no-store`. No on-disk capability
manifest is needed. Empty, malformed, unauthenticated, mismatched, or expired
context is an explicit error with relaunch guidance, even for `--version`;
it never selects ordinary mode. The client permits only IPv4 loopback HTTP,
disables proxy discovery and redirects, and bounds the request time and size.

For `app-server`, the bridge appends live routing overrides at the subcommand's
argument boundary, after desktop/plugin overrides and before any `--` separator.
Other arguments retain their order and values. The preflight goes through this
same bridge and asks the actual engine for effective routing and managed policy.
The real Token Factory key stays in the launcher. Only the temporary local bearer
reaches the owned desktop, and neither it nor the context is an argument or a
durable bridge setting. Each launch creates a new adapter and bearer.

Initial installation claims a new private directory. Existing records and
executables must match before reuse; missing, edited, or symlinked ownership is
an explicit conflict, never permission to overwrite another executable. A setup
write failure removes only the directory created by that attempt. A fully
installed bridge remains usable even if subsequent desktop setup fails.

The later installed-lifecycle ticket owns upgrade and uninstall integration.
Version 1 currently reuses a verified existing bridge rather than replacing it
on every source launch. That ticket must preserve saved executable paths and
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

Every launch prints its private `desktop-sessions/session-*` directory beneath the
tofa configuration directory. Conversation databases, session records, Electron
state and workspace files remain there after exit. Generated routing configuration,
shell-probe files and the temporary model catalog are removed; the local credential becomes
unusable when the adapter closes. The launcher never recursively deletes session
directories. This first slice does not provide a resume command or a shared history
list: [#34](https://github.com/kreuzhofer/nebius-tofa-cli/issues/34) must establish
ordinary-desktop history before a desktop prerelease.

The [#34 contract check](research/desktop-shared-history.md) established an
accepted limitation: Token Factory sessions must remain visible in ordinary
launches, but continuing them may require relaunching through tofa. Token Factory
access lasts only while the launcher is active; switching an existing session to
GPT is not guaranteed. This is not a blocker for #34. Shared-history support
remains incomplete; the synthetic engine checks are not a live desktop
acceptance result.

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

Offline executable/HTTP tests cover isolated state, credential separation, selected
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

This passed on the pinned combination on 2026-09-23. It uses no paid inference;
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
