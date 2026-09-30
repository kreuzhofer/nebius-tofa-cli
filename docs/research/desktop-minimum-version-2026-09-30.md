# Desktop minimum-version compatibility, 2026-09-30

Issue #35 is adapting to the installed desktop after an automatic update. The
user requires a minimum supported version rather than exact release equality
so that a fleet can receive desktop updates without a launcher release for each
build. This note records local first-party evidence; it does not qualify all
future releases or promote experimental model combinations.

## Source identities

The inspected bundle is `/Applications/ChatGPT.app`. Its `Contents/Info.plist`
reports `com.openai.codex`, executable `ChatGPT`, version `26.928.21956`, build
`12404`. SHA-256 identities make this investigation reproducible without using
those hashes as installation gates:

| Source | SHA-256 |
| --- | --- |
| `Contents/Resources/app.asar` | `3bda98f2265ad23677dfe0163d1cc7855beade6bef11d27f830f6663d7658406` |
| `Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex` | `50ac633af64851511f9bbc71032cdae7f1ba20b3234c189687d61ba846c354c5` |
| `Contents/Resources/codex-cli/bin/codex` | `50ab38ba21d0d9f8346f32f41848382f15b556190f3c7a07e885a4fb73e379c8` |
| `.vite/build/application-network-startup-D74LEWDz.js` in ASAR | `9d0317f090ce2b9bc8edb9604a2adc66c8839278fa9d1037dcbf51a5e673bb95` |
| `.vite/build/bootstrap-B7ariqxX.js` in ASAR | `ad9f3da3d96e6713c89b800d1e0c369f8fad1cc20af8233cf7bd906550a2a5fd` |
| `.vite/build/src-ghAWefM3.js` in ASAR | `4ff55fed651b74a13b6cc7f65a47c6f662ffe5475b903cb902cffd545ad1252e` |
| `webview/assets/app-initial-135a4ef2552c.js` in ASAR | `8d0cf4d91cf95805808464d43e06ab3106e03924b4f51020e20f143553e746c3` |

Extracted inspection copies live in
`/private/tmp/tofa35-updated-desktop-source`. Function markers below refer to
these exact source identities; minified names are evidence locators, not public
APIs. The earlier inspection/schema artifacts are
`/private/tmp/tofa41-desktop-source` and `/private/tmp/tofa-55-schema`.

## Engine selection and environment

**Observed source:** In `application-network-startup-D74LEWDz.js`, `Qt()` chooses
`Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex` on macOS. Both that engine
and `Resources/codex-cli/bin/codex` exist, but they are distinct files. `Xs()`
first resolves an explicit `CODEX_CLI_PATH`, then the bundled `Qt()` path.
If the nested executable is absent, `Qt()` returns null; `Xs()` subsequently
tries `Resources/app.asar.unpacked/codex` and a supplied repository
`extension/bin/codex`. It does not fall back to `codex-cli/bin/codex` or a package
manifest. `Zs()` gives a configured `hostConfig.codex_cli_command` precedence over both.
`$s()` builds the native app-server arguments, including
`--analytics-default-enabled`. The local-daemon connection branch requires an
empty `CODEX_CLI_PATH`; a launcher bridge override keeps the stdio route.

**Implementation implication:** Discover the native nested application path for
this layout and retain support for the previous `Resources/codex` layout. An
explicit engine override and an unknown/missing resource layout must remain
observable. Qualify the binary the native desktop would select rather than
assuming the adjacent CLI copy is interchangeable just because versions match.
The native selected binary's parent directory changes too, so verify helper
resource discovery and sandbox behavior through actual-engine tests.

**Observed source:** In `src-ghAWefM3.js`, constants `wp`, `Tp`, `Ep`, and `Dp`
still identify `CODEX_SHELL`, `_SHELL_ENV_DELIMITER_`, the exact NUL-delimited
`printf; command env -0; printf; exit` query, and the three auto-update/tmux
variables used by the existing launcher. In
`application-network-startup-D74LEWDz.js`, `bt()` still supplies `-ilc` for the
interactive Unix probe and `St()` resolves the user's login shell.

**Implication:** The existing narrowly matched zsh environment-probe isolation
has source support on this build. This is not evidence that arbitrary future
shell-probe changes can be ignored; keep isolation checks and errors explicit.

## Configuration and conversation contracts

**Observed source:** `bootstrap-B7ariqxX.js` method `writeModel(t,n,r)` still
sends `config/batchWrite` with `model` and `model_reasoning_effort` upserts,
`filePath:null`, `expectedVersion:null`, `reloadUserConfig:true`; a nonempty
profile prefixes each key with `profiles.<profile>.`.
`setDefaultModelConfig()` delegates to that method.

**Observed source:** The renderer `app-initial-135a4ef2552c.js` still branches on
`status === "okOverridden"`: it restores the cached global configuration and
stores `{model, reasoningEffort, profile}` in a local override. Thus the session
response used by the launcher's CLI-default containment is still recognized.
Actual visible model-selection retention needs UI qualification.

**Observed schema:** Generated schemas from the native nested engine using
`app-server generate-json-schema --experimental` are retained at
`/private/tmp/tofa35-native159-schema-experimental`. All 440 generated JSON files
are structurally equal to the independent artifact at
`/private/tmp/tofa35-engine159-schema`. The following contract files are also
structurally equal to the earlier `/private/tmp/tofa-55-schema/v2` artifacts:

- `ModelListParams.json`, `ModelListResponse.json`.
- `ConfigBatchWriteParams.json`, `ConfigValueWriteParams.json`,
  `ConfigWriteResponse.json`, `ConfigReadResponse.json`.
- `ThreadSettingsUpdateParams.json`, `ThreadSettingsUpdateResponse.json`.
- `ThreadStartParams.json`, `ThreadResumeParams.json`, `TurnStartParams.json`.

The nonexperimental schema projection omits experimental fields. Compare with
matching `--experimental` settings; otherwise apparent removed fields are an
artifact of schema generation rather than an engine regression.

**Implication:** No schema migration is indicated for the current model picker,
configuration response, and thread selection seams. Schema equality cannot
establish unchanged runtime behavior, and it does not capture every upstream
inference request shape. Keep actual-engine regression tests for defaults,
resume/switch, tool continuation, and approval execution gates.

## Catalog and auxiliary requests

**Observed command:** With a disposable `HOME`, `CODEX_HOME`, and working
directory, both installed binaries report `codex-cli 0.159.2`. Both produce
byte-identical `debug models --bundled` output: 11 descriptors, SHA-256
`262dc36e0e7beb290aef8196bd59f42acf51647b8f7b68dafa533ae7742019e6`.
The artifact is `/private/tmp/tofa35-engine159-bundled-models.json`.
The CLI-copy `login status` returned exit 1 and `Not logged in`, confirming that
probe used no native account. No inference or real account catalog request was made; the synthetic loopback
follow-up is recorded below.

**Existing implementation concern:** At inspection time,
`internal/tofa/desktop_catalog_darwin.go` required
`cache.Version == "0.155.0"`. That is an independent exact-version restriction;
changing bundle validation alone would still reject a fresh newer account
catalog. Match cache version to the selected engine's normalized version while
retaining freshness, identity, canonical descriptor comparison and explicit
failure. A follow-up isolated loopback probe reused the repository
`TestDesktopUsesAuthenticatedNativeCatalog` synthetic account convention. The
engine requested `/models?client_version=0.159.2` with the expected synthetic
bearer, exported the supplied `fixture-account-native` descriptor, and wrote
`models_cache.json` with `client_version:"0.159.2"` (full patch version, not
`0.159.0`). The command exited zero with empty stderr. The redacted result is
`/private/tmp/tofa35-engine159-cache-probe.json`. This establishes the cache
serialization contract, not real account refresh availability.

**Observed source:** In `bootstrap-B7ariqxX.js`, `RJ()` generates titles using
feature `thread_title`, schema `AJ`/`jJ` (required title of length 1–36 and
nonempty description), and `HJ()`/`JB()` with low effort, restricted tooling and
an ephemeral generation model. `XB` uses the default
`ephemeralGenerationModel`, supplied in `src-ghAWefM3.js` as `gpt-5.6-luna` with
configuration override support. Other distinct features include
`thread_description` and `thread_summary`.

**Implication:** Source shape remains consistent with the narrow title route;
this does not newly enable DeepSeek titles or authorize generic auxiliary
routing. Captured request tests and visible title behavior remain the evidence
needed for release qualification. Guardian approval inference payloads are not
fully described by app-server request schemas; preserve the strict recognized
review contract and verify allow/deny/failure/cancellation at runtime.

## Minimum-version policy and remaining qualification

**Recommendation/inference:** Separate admission from qualification. Admission
uses a documented minimum desktop/engine version, supported bundle identity and
resource layout, and the existing concrete protocol/ownership checks. Higher
versions must not fail solely for being newer. Exact bundle/engine hashes belong
in immutable qualification evidence, not a fleet-wide allowlist. Version
comparison must be numeric and account for prereleases, with malformed/below-floor
versions producing explicit recovery guidance.

A minimum version cannot promise compatibility with every unknown future release.
Runtime boundary validation should reject a demonstrated incompatible contract
without silently substituting a provider, Guardian, catalog, or engine. Neither
version admission nor these source/schema probes qualifies the installed desktop
UI, account catalog, live model combinations, or native approval enforcement.
The issue #35 workflow still needs those checks before publication.

This note changes no product code, credentials, native profile, or application
state. Research probes launched only isolated engine commands and a loopback server
with generated synthetic credentials; the desktop app was not launched.

## Ownership harness startup observation

A follow-up source audit addresses the ownership harness's empty redirected
stdout while native owner/engine acknowledgements succeed. No application was
launched for this audit.

**Observed source:** `build-flavor-IWwoo-v9.js` (SHA-256
`a23029482b1a5b16b6e049bce139f02b9ce038b1f1f58af4730ffdf2c26bef58`) defines `u` as equality of `env.CODEX_SPARKLE_ENABLED` to the string `false`;
`d(e,t,n,r)` requires `!u(r)` before allowing an updater for a build flavor and
platform. `shouldIncludeSparkle()` and `shouldIncludeUpdater()` use that gate.
In `bootstrap-B7ariqxX.js`, updater class `uCe.initialize()` returns immediately
when `options.enableUpdater` is false, and `initializeUpdater()` also checks the
same option. In `main-BbeJ4AAR.js`, `_Qe()` computes both flags using the gate
and logs an info-level `Launching app` event with `enableUpdater` and
`enableSparkle`. The disable environment variable remains respected.

**Observed source:** Bootstrap function `qxe()` routes each enabled log to both
`Kxe()` (console) and `Wxe()` (rotating files). `Dq()` reads
`CODEX_MAX_LOG_LEVEL`; `info` explicitly admits the startup event, while `warning`
or `error` would filter it. On macOS, `L5()` places files under
`os.homedir()/Library/Logs/<bundle-name-for-build-flavor>`; for production this is
`com.openai.codex`. `H5()` creates UTC `YYYY/MM/DD` subdirectories, and `W5()`
forms names `codex-desktop-<session>-<pid>-t<thread>-i<instance>-<HHMMSS>`, followed
by `-<segment>.log` in `Wxe()`. File writes can complete after engine startup.
The field formatter in `logger-CUwSCKgw.js` still writes booleans as
`enableUpdater=false` and `enableSparkle=false`.

**Harness recommendation:** Set `CODEX_MAX_LOG_LEVEL=info` in the fixture and
poll for that startup event in redirected stdout plus the fixture HOME's log
files whose filename identifies the spawned desktop PID. Assert both false
fields in that event. Do not inspect ordinary-profile logs or accept another
process's startup event. Keep the existing bounded deadline and native
singleton/engine ownership assertions. Empty stdout is not an updater-policy
failure; this audit does not establish why the native launcher redirected or
suppressed that particular stream. Runtime confirmation of fixture log-file
observation remains part of the parent qualification run.

## Startup helper is distinct from the conversation engine

The parent qualification observed a short-lived app-server, an empty process gap,
then the main app-server. `/private/tmp/tofa35-desktop-process-watch.json`
records a helper at about 3.8 seconds, no observed app-server at 4.35 seconds,
and the later bridge/native-engine pair alive at 5.27 seconds. This observation
must not be interpreted as the established conversation engine exiting.

**Observed source:** `application-network-startup-D74LEWDz.js`, `Gc()` (exported
as `readStartupApplicationRequirements`), calls `Gs(e)` without added
configuration overrides, constructs a temporary `Vs` stdio transport, and sends:

```json
{"id":"network-initialize","method":"initialize","params":{"clientInfo":{"name":"codex_desktop","title":"Codex Desktop","version":"<app version>"},"capabilities":{"experimentalApi":true}}}
```

After the initialize response it sends `initialized`, then
`configRequirements/read` with ID `network-requirements`. Its optional sign-out
branch first sends `account/logout` with ID `network-logout`. On completion,
`.finally(()=>c.close())` closes the helper transport. The helper exists to read
application network requirements, not to own the conversation's lifetime.

**Observed source:** In `bootstrap-B7ariqxX.js`, constant `PA` and builder
`FA()` use ID `__codex_initialize__` for the main app-server initialize request.
Its capabilities include extensions and opted-out notifications, and now
`explicitGatewayOauth`. The successful response is processed before
`completeInitialization`; `LA()` emits `initialize_handshake_result`. The earlier
`src-DldfpmrL.js` source also uses main ID `__codex_initialize__` in `IW`/`LW()`.
Thus the known main initialization exchange supplies a semantic lifecycle
boundary across both inspected versions. Client name alone is not established
as a role discriminator.

**Observed source:** Both versions' baseline CLI arguments include
`-c features.code_mode_host=true app-server --analytics-default-enabled`.
New `Gc()` uses those four arguments with no additional overrides. The new main
transport adds `getConfigOverrides`, including the `Qc()` codex-app-tools MCP
switch and `Hpe()` code-review MCP switch (each is a `-c` pair), explaining the
observed eight arguments when optional overrides are absent. The older main
already added the codex-app-tools switch through `Xie()`. Argument counts and
analytics/code-mode flags therefore cannot reliably distinguish lifecycle roles.

**Recommendation:** Let startup helpers use the bridge, but only identify and
monitor the conversation app-server once its known main initialization exchange
is observed (preferably its successful response). An exit before this boundary
must not arm the established-engine-loss shutdown timer. Once a main process is
identified, preserve terminal cancellation for loss of that owned process;
subsequent arbitrary app-server children must not silently replace it. Keep a
bounded startup deadline for cases where main initialization never occurs.

**Historical limitation:** The retained old source has the main handshake and
ordinary transport but lacks imported `bootstrap-C4dRql4x.js`; no old ASAR was
available in the inspected temporary/qualification paths. Absence of the helper
literal from those partial files does not prove the earlier desktop lacked the
helper. Its historical presence remains unverified. The new helper's intentional
short lifetime and the stable main handshake are directly evidenced.
