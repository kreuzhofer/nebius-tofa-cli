# Desktop shared-history contract

Investigation for [#34](https://github.com/kreuzhofer/nebius-tofa-cli/issues/34),
2026-09-23. **#34 is not complete.** This note distinguishes engine capabilities
from the installed desktop's user-facing behavior. Sharing a database alone does
not qualify the required ordinary/Token Factory launch experience.

## Accepted requirement revision

On 2026-09-23, the maintainer explicitly accepted launcher-scoped Token Factory
availability. Tofa sessions must remain visible in ordinary desktop launches,
with their identities, titles, messages, and workspace associations retained.
Continuing a tofa session may require relaunching through tofa; switching an
existing session to GPT is not guaranteed. A missing-provider error while tofa is
inactive is an accepted limitation, **not a blocker for #34**. This supersedes
the earlier requirement to offer an available-provider switch in ordinary mode.

The shared-profile implementation, safe routing lifecycle, and live desktop
verification remain required. The revision accepts unavailable inference while
tofa is inactive; it does not accept hidden or lost conversations, stale routing,
account changes, or silent provider switching.

## Scope and provenance

The inspected client is ChatGPT **26.915.31945 (9922)**, bundle ID
`com.openai.codex`, with bundled engine **0.155.0-alpha.9.2**, on the macOS/arm64
combination gated by the [launcher documentation](../codex-desktop.md). Source
inspection read the installed application archive, not user credentials or
conversation databases. It did not start or modify the ordinary desktop.

Installed archive: `/Applications/ChatGPT.app/Contents/Resources/app.asar`.
SHA-256:
`1f7939c1c781887c167043c4d1d307af3400d324685cfc315dfe2f80e634f483`.
Members were extracted using the archive's own header offsets and sizes and
checked against the archive bytes. The locators below are minified function names
and literal request keys; they apply only to these exact members.

| Archive member | SHA-256 |
| --- | --- |
| `.vite/build/main-DUHZj4_w.js` | `9e8a3bd79c817064f28693ca26aa1378895e07ab2108c78c42d0ea20dac9d66e` |
| `.vite/build/src-C3YaUE83.js` | `14c8c23e8b8dfa874d3fb5a50d54fb28eccf55fb83232c3ab29cb7c0ef0a0472` |
| `.vite/build/bootstrap-DF0QwAxC.js` | `5787d416ccbd7be549251d2c691f9c2579d6960f3ccd470037d97486c93fdf0f` |
| `webview/assets/app-initial-a498f911edeb.js` | `34a75db63c7137eb4caecdba1f36d631c10c7912487e532fd5e9dafb175bb9be` |
| `webview/assets/app-shared-8f4fbb856ceb.js` | `69c6884bfe1d1cbf654da264af1f24d4a6533c25227e221aaef47d96879651cd` |

Engine source is pinned to commit
[`4607249e430dac1c961df4dc615beae88e33cec8`](https://github.com/openai/codex/tree/4607249e430dac1c961df4dc615beae88e33cec8).
The exact commit's Git tree and raw files were fetched for this investigation.
`thread_processor.rs` has Git blob ID
`04c222965227872699ceefac485c96ce219d5183` and SHA-256
`d4be28526badd6068db21c37312be718e51cd42e0a67ffb29665f526f56fd3f1`.
An older cached tree described a different blob; it was not used as provenance.

## Confirmed engine and desktop contracts

**The engine can enumerate different providers in one store.** A nonempty
`thread/list.modelProviders` filters by provider; `[]` includes all providers.
Absent/null defaults to the active provider, except relation-filter queries.
The installed main-process catalog function `M5` explicitly requests `[]` with
`useStateDbOnly:true` and projects each thread's ID, title, working directory,
and provider through `zh`. Some renderer list paths use null instead, so catalog
capability alone is not evidence that every visible list has parity.
[Pinned list implementation](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server/src/request_processors/thread_processor.rs#L5417).

**A normal cold resume preserves recorded provider/model when metadata exists.**
`merge_persisted_resume_metadata` applies the stored model and provider unless
the resume explicitly overrides model, provider, or model reasoning effort.
This uses state-database metadata; it is not a guarantee for arbitrary imported
or incomplete rollout files. The installed main path `Kye` → `Sx` → `nD`
prepares a null provider except for the special Copilot proxy, and its resume
request sends `model:null`. The matching renderer path (`FJt`, shared helper
`Ale`) does likewise. Merely changing the process's default provider therefore
does not intentionally convert existing threads to Token Factory.
[Pinned metadata merge](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server/src/request_processors/thread_processor.rs#L224),
[metadata loading](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server/src/request_processors/thread_processor.rs#L4166).

**A missing recorded provider is an explicit error.** Configuration resolution
looks up the selected provider and returns `Model provider '<id>' not found`
(with backticks around the actual ID in the implementation) when absent.
Removing a launch-only provider definition therefore makes ordinary-mode
continuation unavailable, as accepted above. The intended recovery is relaunching
through tofa with a fresh provider route. Leaving a definition pointing at an
exited adapter would instead retain an unusable endpoint.
[Pinned configuration lookup](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/core/src/config/mod.rs#L3710).

**The inspected desktop model picker does not implement arbitrary provider
switching.** Renderer `jLa`/`PLa` queries `model/list` for the configured provider;
the API accepts pagination and hidden-model controls, not a provider selector.
Renderer `gRa` changes an existing thread through
`updateThreadSettingsForNextTurn` only when its resume state is `resumed`; its
selection carries model, effort, and optionally service tier. The corresponding
`thread/settings/update` API has no provider field. Although `thread/resume`
does accept an explicit provider override, the inspected desktop resume path
does not wire an arbitrary provider choice into that field. Selecting a model
through this path cannot recover a failed resume caused by a missing provider.
This is a concrete limitation of the traced picker/resume integration, not a
claim that the engine is incapable of explicit switching.
[Model-list API](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server-protocol/src/protocol/v2/model.rs#L55),
[thread-settings API](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server-protocol/src/protocol/v2/thread.rs#L236),
[resume API](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server-protocol/src/protocol/v2/thread.rs#L354).

**The picker can write persistent defaults.** When `gRa` does not update an
already-resumed thread, its default path calls `setDefaultModelConfig`.
Main-process `Yye.writeModel` sends `config/batchWrite` with `filePath:null`,
`reloadUserConfig:true`, and edits to `model`/`model_reasoning_effort`, optionally
under `profiles.<name>`. Using an ordinary configuration directory therefore
requires explicit qualification of these writes. Environment-only launch
overrides do not establish that ordinary defaults remain unchanged. This source
path proves the write capability, not that a particular normal user interaction
has already corrupted a default.

**Electron locking differs between ordinary and overridden profiles.** Bootstrap
`uk` requests a single-instance lock for packaged applications when not on macOS,
or when `CODEX_ELECTRON_USER_DATA_PATH` is explicitly set. Ordinary packaged macOS
without that override skips this Electron lock. `pk` chooses the overridden user
data path when present. This prevents treating the current private-profile
lifecycle as proof of safe ordinary-profile reuse. It does not establish whether
other engine or database coordination protects a particular concurrent launch.

## Installed-engine boundary experiment

The [opt-in diagnostics](../../scripts/desktop_history_test.py) start the actual
bundled engine against temporary synthetic homes/workspaces and a loopback
Responses fixture. They use dummy credentials and no paid inference, ordinary
desktop process, or user conversation store.

```sh
TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
  python3 scripts/desktop_history_test.py -v
```

On 2026-09-23, the initial desired ordinary-mode resume failed with
``Model provider `nebius-tofa` not found``.
The original diagnostic **passed: one test**, because it asserted the observed
unavailable-provider behavior explicitly; this is not a passing shared-history
acceptance test. It demonstrated:

1. A synthetic ordinary conversation retained its `openai` provider and
   `fixture-native` model when the engine restarted with Token Factory defaults,
   and continued successfully.
2. A second conversation created with provider `nebius-tofa` and model
   `moonshotai/Kimi-K3` remained listed once alongside the ordinary conversation
   after restarting without that provider definition. `thread/read` retained
   titles, working directory, and complete turns.
3. The desktop-shaped null-model/null-provider resume of that second
   conversation failed explicitly because `nebius-tofa` was unavailable.
4. A direct API request explicitly choosing `openai`/`fixture-native` resumed
   the same conversation and completed a fixture turn. A fourth engine launch,
   again with Token Factory defaults, preserved that explicit choice, both IDs,
   the recorded title, and two complete turns. The resumed response's name was
   null; persisted title preservation was verified separately with `thread/read`.

The four local fixture requests used the expected models in order:
`fixture-native`, `fixture-native`, `moonshotai/Kimi-K3`, `fixture-native`.
All used `/responses` with the synthetic bearer credential.
[Sanitized experiment record](evidence/desktop-shared-history-2026-09-23.json).
The successful direct API provider switch does not supply the missing desktop
UI action or qualify the launcher's shared-profile lifecycle.

Validation also passed `go test -race ./...` (including the opt-in installed-engine
routing check), `go vet ./...`, and 138 tests across the offline Python modules
(33 platform/installed-client skips). The strengthened history diagnostic passed
again after review. Standards and spec reviews each had zero unresolved findings;
the spec review explicitly retained the incomplete feature acceptance below.
Native distribution lifecycle/terminal scripts and live desktop UI qualification
were not run for this diagnostic/documentation change.

The follow-up `test_tofa_history_resumes_when_provider_returns` exercises the
accepted recovery path without changing providers: create a Token Factory
conversation, reopen in ordinary mode and read its history despite failed
resume, relaunch with the Token Factory provider and continue the same ID, then
reopen in ordinary mode again and read both complete turns. This tests the
engine contract; production launcher integration and live UI evidence remain
outstanding. Both installed-engine diagnostics passed on 2026-09-23; see the
[sanitized relaunch evidence](evidence/desktop-shared-history-relaunch-2026-09-23.json).

## Remaining implementation and qualification

Explicit provider switching in the desktop is no longer required for acceptance.
The launcher must reconnect retained Token Factory conversations to a fresh
owned route on relaunch. The installed picker's switching limitation above does
not prevent pursuing that design. Shared-profile routing and lifecycle are not
implemented or qualified by these engine diagnostics.

The following remain **unverified**, rather than proven impossible:

- Ordinary/Token Factory UI list parity, history/title/workspace retention, and
  bidirectional continuation after relaunch, including account/onboarding state.
- Safe shared-profile launches while the ordinary app is already running, live
  thread ownership, consent before interruption, and recovery after crashes.
- Temporary provider-route refresh without stale loopback endpoints, persisted
  defaults changes, credential changes, or weaker managed policy.
- Client upgrades, uninstall preservation, and a sanitized live UI demonstration
  on macOS.

The current [isolated launch](../codex-desktop.md) remains an experimental #32
capability. Its retained per-launch histories do not satisfy #34, and its
existing lifecycle tests must not be represented as shared-history proof.
