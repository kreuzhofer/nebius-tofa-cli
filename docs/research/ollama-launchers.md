# Ollama launchers: behavior, reuse, and boundaries

Research date: 2026-09-21. Investigation: [Trace Ollama launcher behavior and identify reusable MIT-licensed code](https://github.com/kreuzhofer/nebius-tofa-cli/issues/2).

## Finding

Ollama is a useful reference implementation for an agent launcher, especially its Claude Code environment injection, Codex CLI provider isolation, integration registry, installation checks, and configuration ownership. Its desktop integrations are substantially more than launch commands. ChatGPT/Codex desktop uses a local routing service and merged model catalog; Claude Desktop uses a dedicated local gateway. **Copying those integrations does not provide a direct-to-Token-Factory desktop connection.** [CLI Claude source][claude], [CLI Codex source][codex], [ChatGPT source][chatgpt], [Claude Desktop source][desktop].

The repository is MIT licensed. Selective code reuse is feasible with the license notice retained; importing the whole launcher package would also import Ollama server/model/catalog assumptions and separately licensed dependencies. Recommend reimplementing a small integration boundary and adapting selected helpers, rather than forking the entire application. [License][license], [dependency manifest][gomod].

Current scope is research and decisions for a first prototype using direct Token Factory routes. Proxy development is deferred. Protocol compatibility with Token Factory is a separate investigation; the injection parameters below describe Ollama, not verified Nebius configurations.

## Evidence and reproducibility

- Primary inspected snapshot: Ollama main commit [`6383a0fa9cbf97494b847226e189f6e36b401a08`](https://github.com/ollama/ollama/commit/6383a0fa9cbf97494b847226e189f6e36b401a08), dated 2026-09-18. All Ollama source links below pin this commit.
- Latest stable GitHub release returned during research: [`v0.34.2`](https://github.com/ollama/ollama/releases/tag/v0.34.2), published 2026-09-15, tag commit [`dfabde4539e42ba1e1eab50a3a50b88aea7958a0`](https://github.com/ollama/ollama/commit/dfabde4539e42ba1e1eab50a3a50b88aea7958a0). The development snapshot is not the released binary.
- Inspected a public source checkout, the release-to-main diff, source tests, first-party integration docs, and exact-version dependency licenses. No installed agent was launched; no real account/configuration was modified; no inference, paid API call, or cross-platform runtime test was performed. “Source” below means implementation inspection, not end-to-end verification.
- Release-to-main differences include Codex reasoning controls derived from model capabilities, an additional model-show lookup during launch, and desktop routing/restore refinements. `claude.go`, `claude_desktop.go`, the registry, and integration documentation are unchanged between these two revisions. Do not promise main-only behavior for v0.34.2. [Pinned comparison](https://github.com/ollama/ollama/compare/dfabde4539e42ba1e1eab50a3a50b88aea7958a0...6383a0fa9cbf97494b847226e189f6e36b401a08).

## The common launch flow

The Cobra command accepts `ollama launch [INTEGRATION] [-- EXTRA_ARGS...]`, `--model`, `--config`, `--restore`, and `--yes`/`-y`. With no integration, it opens the same interactive menu as bare `ollama`. Arguments after `--` go to the integration. The registry resolves aliases and installation checks; runners expose narrower optional interfaces for configuration, model discovery, restoration, and platform support. [Command dispatch][dispatch], [registry][registry].

Before an ordinary launch, Ollama checks its own server heartbeat and may start its own desktop application. On Linux, the fallback instead tells the user to run `ollama serve`. Restore bypasses this server check. This coupling must be replaced with remote endpoint/account checks in this project; setting `OLLAMA_HOST` to an arbitrary API URL would not make the rest of the launcher API-compatible. [Server heartbeat][heartbeat], [Linux start fallback][linuxstart], [model inventory][inventory].

Model selection considers `--model`, stored integration preferences, usability checks, and interactive selection. The inventory comes from Ollama's model listing, recommendations, model metadata, cloud account state, and optional pulls. Preferences live in `~/.ollama/config.json` under `integrations`, with model lists, aliases, onboarding state, and optional auto-mode state; the file also records the last model/selection. This is independent of each agent's own configuration. [Target resolution][selection], [preference persistence][config], [inventory][inventory], [readiness and pulls][models].

Interactive operation prompts; `--yes` permits confirmations and missing-model pulls. Noninteractive operation without `--yes` fails when an answer/download is needed. A headless `--yes` launch generally requires an explicit model except for managed autodiscovery integrations. That distinction is useful for CI and scripting, but downloading local Ollama models is irrelevant to a remote-only launcher. [Launch policy][policy], [dispatch][dispatch].

**`--config` nuance:** the help describes configuration without launching, but single-model runners flow through `launchAfterConfiguration`, which asks whether to launch now. For Codex CLI, the actual agent profile/catalog write is inside `Run`; declining that question persists the Ollama model preference but does not run `ensureCodexConfig`. This is source-inferred behavior worth defining explicitly in our own UX. [Single-runner flow][singleflow], [configuration continuation][continuation], [Codex Run][codex].

## Integration matrix

| Surface | Ollama integration key | Discovery/install | Routing mechanism | Source platform boundary |
| --- | --- | --- | --- | --- |
| Claude Code terminal | `claude` | PATH plus native-install fallback directories; optional official installer after confirmation | Child-process Anthropic variables and `--model` | Installer cases macOS/Linux/Windows; no desktop dependency |
| Codex CLI | `codex` | PATH, then `codex --version`; no automatic install hook | Named profile, catalog, CLI overrides, Responses API | No runner OS gate; native platform execution still requires testing |
| ChatGPT desktop, Codex mode | `chatgpt`; aliases `codex-app`, `codex-desktop`, `codex-gui` | macOS app bundle or Windows executable/app ID; manual installation link | Persistent root config plus catalog and local router | macOS and Windows; Linux rejected |
| Claude Desktop | hidden `claude-desktop`; alias `claude-app` | App/profile discovery; manual installation link | Third-party gateway profile and separate local gateway | Enabling is macOS-only; CLI permits only restore; Windows cleanup has a guard discrepancy discussed below |

Sources: [registry][registry], [Claude][claude], [Codex][codex], [ChatGPT][chatgpt], [Claude Desktop][desktop], [command guard][dispatch]. “No OS gate” is not a claim of verified parity.

## Claude Code: exact launch contract

`findPath` first uses `exec.LookPath("claude")`, then checks `~/.local/bin/claude` and `~/.claude/local/claude`; on Windows the fallback filename is `claude.exe`. The resulting path is executed directly with `--model MODEL` followed by extra arguments. Current working directory is inherited. Stdin, stdout, and stderr are attached to the calling terminal, and the parent waits for the child. [Claude implementation, lines 24–74][claude].

The runner appends these values to its child environment:

| Variable | Ollama value |
| --- | --- |
| `ANTHROPIC_BASE_URL` | `envconfig.Host().String()`; normally `http://127.0.0.1:11434` |
| `ANTHROPIC_API_KEY` | Empty string, intentionally clearing an inherited API key |
| `ANTHROPIC_AUTH_TOKEN` | `ollama` |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Selected model |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Selected model |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Selected model |
| `CLAUDE_CODE_SUBAGENT_MODEL` | Selected model |
| `CLAUDE_CODE_ATTRIBUTION_HEADER` | `0` |
| `CLAUDE_CODE_TOTAL_TOKENS_REMINDER` | `off` |
| `DISABLE_ERROR_REPORTING` | `1` |
| `DISABLE_FEEDBACK_COMMAND` | `1` |
| `CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY` | `1` |
| `CLAUDE_CODE_AUTO_COMPACT_WINDOW` | Added for a recognized cloud model when Ollama has its context limit |

Sources: [environment construction][claude], [host parsing][host]. Go's command environment uses the last occurrence of a duplicate environment key, so these appended entries override the inherited values. This establishes child-process isolation, not the complete precedence between Claude settings files, managed policies, and Claude's own arguments. [Go `os/exec.Cmd.Env`](https://pkg.go.dev/os/exec#Cmd).

`OLLAMA_HOST` controls Ollama's host parser. Claude uses `Host()` directly, whereas Codex uses `ConnectableHost()`, which replaces unspecified bind addresses such as `0.0.0.0` with loopback addresses. Do not reuse this local-server parsing as a generic Nebius endpoint policy. [Host functions][host].

When Claude is missing, Ollama checks prerequisites and asks to install. On macOS/Linux it runs `bash -c 'curl -fsSL https://claude.ai/install.sh | bash'`; on Windows it runs `powershell -NoProfile -ExecutionPolicy Bypass -Command 'irm https://claude.ai/install.ps1 | iex'`, then searches again. These are upstream agent installers, separate from distributing our launcher. [Installer functions][claude].

The runner does not persist these Anthropic variables into shell startup files or Claude settings. It has no `Restore` implementation because its routing mutation is scoped to the launched process; Ollama's own remembered model remains. Ollama documents an Anthropic-compatible API and warns that some advanced tool controls/hosted search capabilities are incomplete. Merely accepting OpenAI chat completions is insufficient for this integration. [Claude source][claude], [Claude integration docs][claudedocs].

**Reusable:** discovery fallbacks, explicit child environment, model-tier mapping, direct argv construction, and installation capability checks. **Replace:** auth token, endpoint selection, cloud naming/context assumptions, branding, and automatic-install policy. Avoid copying telemetry or model-tier overrides without deciding whether those settings are part of our product contract.

## Codex CLI: exact launch contract

Ollama requires `codex` on PATH and calls `codex --version`; `checkCodexVersion` compares against **0.134.0** using `golang.org/x/mod/semver`. The registry gives installation guidance but has no Codex `EnsureInstalled` hook, so normal launching does not automatically run npm. [Version check and runner][codex], [registry][registry].

The runner writes:

1. `~/.codex/ollama-launch.config.toml`, containing root `model`, `model_provider = "ollama-launch"`, `model_catalog_json`, and `[model_providers.ollama-launch]` with `name = "Ollama"`, `base_url = "<connectable Ollama host>/v1/"`, `wire_api = "responses"`.
2. `~/.codex/model.json`, a one-model catalog. It includes model slug/name, context window, input modalities, visibility, truncation policy, base instructions, tool/reasoning flags, and supported reasoning levels. Unknown context falls back to 128,000; known Ollama metadata/cloud limits and certain local context overrides can replace it.
3. Migration cleanup may remove an old root `profile = "ollama-launch"` and old `[profiles.ollama-launch]` section from `~/.codex/config.toml`, using backup writes. Ordinary current CLI profile creation does not set the root app provider/model.

Sources: [`ensureCodexConfig`, path helpers, profile writer, catalog builder][codex]. These helpers resolve the OS user home plus `.codex`; they do **not** honor an inherited `CODEX_HOME` for their write paths. That is a portability/isolation concern to fix in our implementation, not a convention to copy unquestioningly.

The final argv is equivalent to:

```text
codex --profile ollama-launch
  -c 'model_provider="ollama-launch"'
  -c 'model_providers.ollama-launch.name="Ollama"'
  -c 'model_providers.ollama-launch.base_url="<host>/v1/"'
  -c 'model_providers.ollama-launch.wire_api="responses"'
  -c 'model_catalog_json="<absolute home>/.codex/model.json"'
  -m <selected-model>
  <extra-arguments>
```

This is an argv illustration, not a copy-paste multiline shell command. The child inherits the environment with `OPENAI_API_KEY=ollama` appended. Extra arguments attempting to replace profile, model, catalog, or provider configuration are rejected. This is stronger conflict handling than the Claude runner, which simply appends extra args. Standard streams and working directory are inherited; execution waits for the child. [Argument validation and `Run`][codex].

Ollama's profile does not set a provider `env_key` for a paid upstream credential. The placeholder key is for Ollama's own compatibility service; replacing that placeholder alone is not a complete Nebius provider configuration. Exact supported Codex auth fields and Token Factory's Responses support belong in the separate compatibility investigation. [Provider writer][codex].

`ollama launch codex --restore` removes the CLI profile and removes `model.json` unless root config still references it. It deliberately does not reset unrelated root config or remove desktop settings. This restore is allowed even if Codex itself has been uninstalled. Restoring is explicit: the profile/catalog persist after an ordinary child process exit. [Restore implementation][codex], [restore tests][codextests].

**Reusable:** provider isolation, conflicting-argument rejection, version gating, catalog shape, and ownership-aware cleanup. **Replace:** hardcoded directories/profile names, model limits, capability defaults, auth behavior, and every dependency on Ollama renderers/recommendations. A Nebius model's context, vision, tool and reasoning capabilities must come from a verified catalog, not inherited Ollama defaults.

## ChatGPT desktop / Codex app: persistent routing, not ordinary Chat

Ollama's documentation explicitly distinguishes the surfaces: its models appear in **Codex mode** inside ChatGPT desktop; ordinary Chat and voice retain their usual ChatGPT connection. Native models retain their own provider, and sessions remain managed by Codex. This investigation therefore does not support a claim that arbitrary ChatGPT consumer chats can use a custom endpoint. [ChatGPT integration documentation][chatgptdocs].

The `CodexApp` runner supports macOS and Windows, rejects Linux, and rejects extra args. macOS discovery checks `/Applications/ChatGPT.app`, `/Applications/Codex.app`, then the corresponding `~/Applications` paths, with bundle-ID discovery as a fallback. Windows searches `%LOCALAPPDATA%` application paths (ChatGPT/Codex, including `Programs`, `OpenAI`, and `app-*` variations), running process paths, and registered Start app IDs. [Platform/discovery implementation][chatgpt].

Configuration steps are extensive:

- Save previous root settings to `~/.ollama/launch/codex-app-restore.json`.
- Obtain the native Codex model catalog using the app-bundled Codex executable, falling back to PATH. Run `codex debug models` in a temporary `CODEX_HOME`, copying `auth.json` and `models_cache.json` with mode 0600 when present; fallback to the cache, then `codex debug models --bundled`. It strips inherited `CODEX_HOME`, `OPENAI_API_KEY`, and `CODEX_API_KEY` for discovery. The subprocess has a 30-second deadline.
- Write `~/.codex/ollama-launch-models.json` (combined native + selected Ollama models) and `~/.codex/ollama-launch-codex-routing.json` (Ollama-only routing allow-list). The latter determines where requests are sent; the combined picker is not the routing authority.
- Create `~/.codex/auth.json` only if absent, with `auth_mode: "apikey"` and `OPENAI_API_KEY: "ollama-local-codex"`, using exclusive creation and mode 0600. Existing login material is preserved. This sentinel is only for the local router.
- Edit `~/.codex/config.toml`: select the model/catalog, set `openai_base_url` to `<connectable Ollama host>/api/codex/v1`, remove explicit root `model_provider` and old managed profiles, and merge desktop thinking options. Leaving the provider unset preserves native account controls. Back up changed config.

Sources: [configuration, native catalog, auth and path implementations][chatgpt], [router constants and routing implementation][router]. This source does copy account material to a private temporary directory for catalog discovery; it is not an instruction for our prototype to do so.

On macOS, launching uses `open` with the located app or `-b com.openai.codex`; when configured it also opens `codex://threads/new?mode=codex`. On Windows it uses PowerShell `Start-Process` with either a safely quoted executable path or `shell:AppsFolder\<app-id>`. The launcher returns after opening the app, not when the desktop app exits. A running app triggers a restart confirmation; quitting uses AppleScript on macOS or `CloseMainWindow()` on Windows, then waits up to five seconds and reopens. The restart spinner supports cancellation. [Open/restart functions][chatgpt].

The local router distinguishes native models from Ollama models, handles protocol/routing details, and refuses to forward its local-only sentinel to OpenAI. It is part of Ollama's server, not an env-only launcher feature. Replacing its URL with Token Factory would also redirect requests that need native services and would lose this routing logic. [Router source][router].

Restore rewrites only still-managed root configuration, restores recorded settings, preserves a native model the user subsequently selected, removes unneeded owned catalogs/profile state, and removes auth only when it matches the exact local sentinel. It then asks to restart the desktop app if necessary. No target application is uninstalled. [Restore logic][chatgpt].

**Current prototype implication:** use this as future desktop design evidence. Under the direct-only scope, do not advertise Ollama-equivalent mixed native/Nebius model support unless a separate, supported direct-provider integration proves it possible. The executable discovery and restart patterns can still be reused without importing routing.

## Claude Desktop: gateway profile and macOS-first support

The CLI pre-run hook rejects enabling `claude-desktop` and directs users to its `--restore` command. Enabling is driven from the native Ollama app. The integration probes a gateway at **`http://127.0.0.1:11435`** with a two-second timeout; the regular Ollama server address is not used as this profile's gateway. `Supported()` only allows macOS. [CLI guard][dispatch], [Claude Desktop source][desktop], [gateway source][gateway].

On macOS, the application is `/Applications/Claude.app` or `~/Applications/Claude.app`. Configuration roots are `~/Library/Application Support/Claude` and `~/Library/Application Support/Claude-3p`. The launcher writes:

- `claude_desktop_config.json` under both roots, setting `deploymentMode: "3p"`.
- `Claude-3p/configLibrary/_meta.json`, selecting profile ID `00000000-0000-4000-8000-000000000114` and adding its display entry.
- `Claude-3p/configLibrary/<profile-id>.json`, merging the following fields and deleting a prior explicit `inferenceModels` list:

| Field | Value |
| --- | --- |
| `inferenceProvider` | `gateway` |
| `inferenceGatewayBaseUrl` | `http://127.0.0.1:11435` |
| `inferenceGatewayApiKey` | `ollama` |
| `inferenceGatewayAuthScheme` | `bearer` |
| `deploymentDisplayName` | `Ollama` |
| `chatTabEnabled` | `true` |
| `disableDeploymentModeChooser` | `true` |
| `coworkEgressAllowedHosts` | `["*"]` |
| `disableEssentialTelemetry` / `disableNonessentialTelemetry` | `true` / `true` |
| `autoModeEnabled` | Saved preference, defaulting to `true` |

Sources: [profile roots, writers, and preferences][desktop]. These values show what Ollama does; they are not proposed defaults for this project. In particular, unrestricted egress and plaintext credential fields need independent product decisions if desktop support is revisited.

It opens Claude using `/usr/bin/open <app-path>`. A running app requires restart consent. It waits for exit (up to 30 seconds), **reapplies the configuration after shutdown**, then reopens, because Claude writes settings as it exits. Restore sets normal/third-party deployment modes to `1p`, removes the Ollama metadata entry and managed gateway/provider/telemetry/egress fields, and re-enables the deployment chooser. The profile JSON can remain; notably the cleanup list does not delete `inferenceGatewayApiKey` or `chatTabEnabled`. This is a reset-to-usual-mode operation, not a byte-for-byte rollback of every prior field. [Restart and restore implementation][desktop].

Windows discovery/cleanup paths exist (`%LOCALAPPDATA%/Claude`, `Claude Nest`, `Claude-3p`, `Claude Nest-3p`, and several executable locations). `claudeDesktopRestoreSupported` allows Windows, but the generic `restoreIntegration` calls `EnsureIntegrationInstalled`, which invokes `Supported()` first; `ClaudeDesktop` does not implement `SkipRestoreInstallCheck`. **Source-inferred discrepancy:** CLI restore appears to be rejected on Windows before reaching the Windows-aware restore body. Do not describe Windows restoration as verified support based on the inner helper alone. [Restore dispatch][restore], [installation check][installcheck], [desktop support guards][desktop].

Ollama's docs describe macOS support, Windows as forthcoming, gateway-backed app setup, and restoring usual configuration when Ollama quits. The source has corresponding `RestoreForShutdown` handling. [Claude Desktop docs][desktopdocs], [shutdown helper][desktop].

**Current prototype implication:** defer parity with this gateway-backed integration. Reusable future lessons are profile ownership, discovery, graceful restart, reapplying writes after app shutdown, and cleanup that protects user-managed settings.

## Process behavior and file safety

For the two terminal runners, `exec.Command(...).Run()` waits with inherited standard streams. Neither adds a dedicated signal forwarding loop, process-group setup, cancellation context, or child-exit-code translation. A nonzero child error reaches `main`, which calls Cobra `CheckErr`; Cobra v1.7.0 prints the error and exits **1**. Therefore do not assume `ollama launch` preserves an arbitrary child exit code. Ctrl+C behavior depends on the terminal/OS and needs direct tests if we want a clear cross-platform contract. [Claude Run][claude], [Codex Run][codex], [Ollama main][main], [Cobra CheckErr][cobraexec].

The shared `WriteWithBackup` helper compares unchanged content, saves the old file, writes a temporary file in the target directory, syncs/closes it, and renames it into place. Backups normally live in `~/.ollama/backup`, optionally below an integration directory; it retains the five newest backups per basename. Backup names use second-resolution timestamps. This is a useful starting point, not a concurrent transaction system: multiple changes in one second can share a backup name, and similarly named files without an integration subdirectory can collide. There is no locking/rollback transaction across all files in the desktop flows. [File helper][fileutil].

Recommended improvements for this project are explicit write ownership, unique backup names, concurrency control where state is shared, dry-run previews for persistent changes, and tests using temporary homes/fake executables. These are recommendations based on the inspected code, not claims that existing Ollama behavior has been runtime-tested here.

## License and reuse assessment

The root license is **MIT**, copyright Ollama. It permits copying, modification, distribution, sublicensing and sale subject to retaining the copyright and permission notice in copies or substantial portions. There is no source-publication requirement in that license. Keep the full upstream license text in third-party notices when adapting substantial source and mark the source revision and our changes. This is a reading of the published license, not an assessment of rights in every bundled asset or third-party application. [Ollama license][license].

No separate license header or nested license was found in the inspected `cmd/launch` or `cmd/internal/fileutil` files. Relevant imported dependency versions/licenses checked independently:

| Dependency | Inspected version | License | Relevance |
| --- | --- | --- | --- |
| `github.com/spf13/cobra` | v1.7.0 | Apache-2.0 | Command parsing/dispatch; preserve its license and applicable notices, observe modification notice terms when modifying Apache source |
| `github.com/pelletier/go-toml/v2` | v2.2.2 | MIT | Codex TOML parsing |
| `github.com/charmbracelet/bubbletea` | v1.3.10 | MIT | Launcher/TUI and restart spinner |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | MIT | Terminal styling |
| `golang.org/x/mod` | v0.30.0 | BSD-3-Clause | Codex version comparison |

Sources: [Ollama manifest][gomod], [Cobra license][cobralicense], [TOML license][tomllicense], [Bubble Tea license][tealicense], [Lip Gloss license][liplicense], [x/mod license][modlicense]. This is a focused audit of plausible reuse dependencies, not a complete transitive dependency/license inventory. A packaged binary needs notices for its actual dependency closure. The root MIT license does not relicense dependencies, model weights, target desktop binaries, logos, trademarks, or third-party service terms.

| Candidate | Recommended use | Coupling / changes needed |
| --- | --- | --- |
| `Claude.findPath`, argument/environment construction | Adapt small functions and table-driven test cases | Replace host/token/model policy and platform installer choices |
| Registry + `Runner`/optional interfaces | Borrow the design; create a small local implementation | Whole registry knows many irrelevant products and install policies |
| Codex config conflict checks and profile ownership | Adapt narrowly after checking the supported Codex version | Hardcoded `.codex`, Ollama provider/catalog names, newer profile semantics |
| `fileutil.WriteWithBackup` and tests | Adapt with project-specific paths and concurrency fixes | Stdlib-only implementation; backup namespace/timestamp limitations |
| Codex model catalog builder | Reimplement against verified Token Factory capabilities | Main snapshot reaches Ollama `api`, renderers, reasoning normalization and desktop helpers |
| Desktop discovery/restart/restore helpers | Retain as future reference; adapt after desktop scope is reopened | OS scripting, app identity changes, local proxy/gateway state, auth/profile migration |
| Entire `cmd/launch` package | Do not import wholesale | Ollama model server, account state, local/cloud model inventory, TUI, catalog, proxy, config internals |

Sources: [Claude][claude], [registry][registry], [interface definitions][interfaces], [Codex][codex], [file helper][fileutil], [desktop implementations][chatgpt]. `cmd/internal/fileutil` is also an internal Go package, so another module cannot simply import it as a public library; adapting its MIT source is a different approach. [Go internal-package rule](https://go.dev/doc/go1.4#internalpackages).

## Decisions this research supports, and remaining evidence

1. Preserve the familiar `launch <agent>`, model picker, remembered preference, explicit model flag, and `--` argument passthrough. These are independent of Ollama's local inference engine.
2. Keep runtime-free launcher distribution separate from installing the target agents. Auto-installing upstream agents is optional and introduces their own OS/runtime/install requirements.
3. Use terminal integrations as the simplest direct-route candidates. Gate each one on the actual required API and authentication contract; this investigation alone approves none against Token Factory.
4. Treat ChatGPT desktop's Codex mode separately from regular ChatGPT, and treat Claude Desktop separately from Claude Code. Defer gateway/router-dependent parity while proxies are out of scope.
5. Reuse small MIT helpers and behavioral tests where they reduce mistakes, keeping source/notice provenance; avoid importing Ollama's whole dependency graph.

Before implementation acceptance, resolve Token Factory's supported protocols and auth, agent-specific supported settings/precedence, a model-capability source, minimum supported agent versions, and platform test coverage. Test process exit codes/signals, inherited `CODEX_HOME`, config collisions, user edits during restore, and restart cancellation with fake processes before any real-account integration test. No real-agent success or live Nebius compatibility is claimed by this report.

[license]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/LICENSE
[gomod]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/go.mod
[main]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/main.go
[claude]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/claude.go
[codex]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/codex.go
[codextests]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/codex_test.go
[chatgpt]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/codex_app.go
[desktop]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/claude_desktop.go
[dispatch]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L276-L411
[interfaces]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L144-L250
[selection]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L969-L1019
[policy]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L84-L124
[restore]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L525-L543
[singleflow]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L725-L742
[continuation]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/launch.go#L1496-L1510
[registry]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/registry.go
[installcheck]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/registry.go#L528-L561
[heartbeat]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/cmd.go#L2165-L2179
[linuxstart]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/start_default.go
[config]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/config/config.go
[inventory]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/model_inventory.go
[models]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/models.go
[host]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/envconfig/config.go#L20-L82
[fileutil]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/internal/fileutil/files.go
[router]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/internal/proxy/codex_desktop.go
[gateway]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/internal/proxy/claude_desktop.go
[claudedocs]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/docs/integrations/claude-code.mdx
[chatgptdocs]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/docs/integrations/chatgpt.mdx
[desktopdocs]: https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/docs/integrations/claude-desktop.mdx
[cobraexec]: https://github.com/spf13/cobra/blob/v1.7.0/cobra.go#L228-L233
[cobralicense]: https://github.com/spf13/cobra/blob/v1.7.0/LICENSE.txt
[tomllicense]: https://github.com/pelletier/go-toml/blob/v2.2.2/LICENSE
[tealicense]: https://github.com/charmbracelet/bubbletea/blob/v1.3.10/LICENSE
[liplicense]: https://github.com/charmbracelet/lipgloss/blob/v1.1.0/LICENSE
[modlicense]: https://github.com/golang/mod/blob/v0.30.0/LICENSE
