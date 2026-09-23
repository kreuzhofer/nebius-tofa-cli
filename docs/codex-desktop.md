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

The bundled engine resolves configuration in that isolated home/workspace before
launch. The launcher checks the effective model, provider, endpoint, authentication
channel, model catalog, and database/log locations. Unexpected provider headers or
authentication settings fail closed. Managed routing, model, residency, credential
storage, or state-location requirements are currently rejected explicitly;
they are never rewritten. Other enforced policy remains the engine's responsibility.
The original HOME is retained for native policy discovery. The preflight uses
the engine's [`config/read` and `configRequirements/read` APIs](https://learn.chatgpt.com/docs/app-server);
configuration and requirements are [separate layers](https://learn.chatgpt.com/docs/config-file/config-basic).

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
state and workspace files remain there after exit. Generated routing configuration
and the temporary model catalog are removed, and the local credential becomes
unusable when the adapter closes. The launcher never recursively deletes session
directories. This first slice does not provide a resume command or a shared history
list: [#34](https://github.com/kreuzhofer/nebius-tofa-cli/issues/34) must establish
ordinary-desktop history before a desktop prerelease.

## Limitations and verification

Automatic thread titles can fail on the existing tools-plus-JSON-schema restriction;
[#33](https://github.com/kreuzhofer/nebius-tofa-cli/issues/33) owns that follow-up.
The recognized non-strict automatic-review workaround remains unchanged. Native
auxiliary model IDs, compaction endpoints, web search, account-backed services, and
Chat/Work/voice are not qualified. No `--direct`, Guardian evaluation model, profile,
or arbitrary desktop argument passthrough is exposed.

Offline executable/HTTP tests cover isolated state, credential separation, selected
model streaming, auxiliary rejection, managed/effective routing conflicts, startup
and exit errors, engine loss, cancellation, adapter failure, and owned-worker cleanup.
The desktop executable tests run on the exact gated macOS/architecture combination;
other hosts skip them. Portable launcher/adapter regressions still run normally.
To additionally exercise the actual bundled engine against dummy offline settings:

```sh
TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
  go test ./internal/tofa -run '^TestDesktop' -count=1
```

This optional test starts the real engine but a fixture desktop executable; it does
not perform paid inference. The earlier human-operated streaming/tool/continuation
proof and its limitations remain in the [feasibility evidence](research/codex-desktop-feasibility.md).

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
