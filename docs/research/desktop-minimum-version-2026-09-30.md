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
