# Codex desktop feasibility on macOS

Research date: 2026-09-22. Ticket: [#29](https://github.com/kreuzhofer/nebius-tofa-cli/issues/29).

## Finding and evidence boundary

The installed **ChatGPT desktop app, Codex mode**, completed streaming, a real shell-tool round trip, and a continued conversation against Token Factory Kimi-K3 through the existing launcher Responses request adapter. This establishes a bounded, reversible macOS feasibility route. It does **not** qualify the whole desktop integration: automatic thread-title generation failed explicitly in the existing adapter, and auxiliary/native routing still needs a product decision.

The test used separate Codex and Electron state, no copied OpenAI login, and human operation of the scratch window. Three paid inference requests completed; the final answer recalled the command's marker. The last response's observer recorded a late, unclassified transport error, so final completion is corroborated by the owned client's session record and the human, not a pristine final wire trace. [Sanitized evidence](evidence/codex-desktop-macos-2026-09-22.json).

Ordinary Chat/Work/voice, account migration, Windows execution, and vendor-supported Nebius use were not established. No production launcher or broad adapter change was implemented.
## Exact combination and naming

| Component | Inspected combination |
| --- | --- |
| Application | `/Applications/ChatGPT.app`; `CFBundleName = ChatGPT` |
| Bundle identifier | `com.openai.codex` |
| Version / build | `26.915.31945` / `9922` |
| Bundled engine | `Contents/Resources/codex --version`: `codex-cli 0.155.0-alpha.9.2` |
| Platform | macOS `26.6.2` (`25G83`), `arm64` |
| Mode | Codex local conversation, operated manually in the separate window |
| Selected model | `moonshotai/Kimi-K3`; synthetic offline phase, then real Token Factory inference |
| Account in experiment | No OpenAI account login, copied credential, or desktop `auth.json`; real Token Factory credentials supplied only by the already authenticated launcher |

These values came from the installed bundle's `Info.plist`, `package.json`, executable version, and macOS version utilities. `/Applications/Codex.app` was absent. The bundle's package name remains `openai-codex-electron`. This resolves the naming ambiguity in [the earlier client contracts](client-contracts.md): on this machine “Codex desktop” is the Codex surface within the installed ChatGPT app, not an additional `Codex.app`. OpenAI's [current desktop documentation](https://learn.chatgpt.com/docs/app) distinguishes ChatGPT Chat/Work from Codex. Ollama likewise [identifies Codex mode inside ChatGPT desktop](https://docs.ollama.com/integrations/chatgpt).

OpenAI documents ChatGPT and OpenAI API-key sign-in for local desktop work, with narrower features for API-key users. Its custom-provider contract separately supports environment credentials with `requires_openai_auth = false`. The local custom-provider experiment proceeded without those OpenAI sign-ins; this does not establish entitlement to account-backed features. Scratch startup reported sign-in requirements for remote control, push notifications, and remote plugin details. [Authentication documentation](https://learn.chatgpt.com/docs/auth).

## Configuration, launch, and ownership

Bundled application code was inspected read-only; it is version-specific implementation evidence, not a promised public launcher API. The `app.asar` SHA-256 was `1f7939c1c781887c167043c4d1d307af3400d324685cfc315dfe2f80e634f483`. Reproduction locators within that archive:

- `.vite/build/bootstrap-DF0QwAxC.js`: `pk` uses `CODEX_ELECTRON_USER_DATA_PATH` for Electron user data; `uk` enables a distinct instance lock when an explicit path is supplied.
- `.vite/build/main-DUHZj4_w.js`: `OE` preserves explicitly supplied `CODEX_HOME` across shell environment loading when the Electron override exists. The bundled demo launcher also passes both variables and `--user-data-dir` to a separate app launch.
- `.vite/build/src-C3YaUE83.js`: `gM`/`_M` resolve the supplied Codex home; `EQ` constructs the bundled app-server invocation. It adds `features.code_mode_host=true` and maps `CODEX_APP_SERVER_OPENAI_BASE_URL` / `CODEX_APP_SERVER_CHATGPT_BASE_URL` to command-line config overrides. No desktop `--profile` forwarding contract was established.

The test launched the exact app executable as an owned subprocess with `CODEX_HOME=<scratch>/codex`, `CODEX_ELECTRON_USER_DATA_PATH=<scratch>/electron`, and `--user-data-dir=<scratch>/electron`. The working directory was an empty scratch workspace. It created its own Electron Local State and Codex databases and spawned the app-bundled engine. The running ordinary application was not restarted. Setting only Chromium's user-data directory would not have established Codex state isolation.

The scratch `config.toml` used this shape, with an ephemeral loopback port:

```toml
model = "moonshotai/Kimi-K3"
model_provider = "tofa-desktop-fixture"
cli_auth_credentials_store = "file"
web_search = "disabled"

[model_providers.tofa-desktop-fixture]
name = "TOFA Desktop Offline Fixture"
base_url = "http://127.0.0.1:<port>/v1"
env_key = "TOFA_DESKTOP_FIXTURE_KEY"
wire_api = "responses"
requires_openai_auth = false
supports_websockets = false
request_max_retries = 0
stream_max_retries = 0
```

The offline fixture injected only a dummy credential and supplied no model catalog. The later live phase used the launcher-generated Kimi catalog and its temporary local-adapter credential. The desktop accepted that catalog without a missing-model-metadata warning. Neither native fallback metadata nor an Ollama model descriptor was substituted for the launcher's provider snapshot.

The general Codex configuration order is command-line overrides, trusted project config, selected profile, user config, cloud defaults, system config, built-ins; managed requirements are separate. Provider/authentication routing keys are excluded from project config. The desktop's environment-to-CLI mapping can consequently override corresponding file settings. This test used empty scratch user/project state and did **not** exercise every conflicting layer or managed-policy combination. A launcher must inspect effective configuration and preserve enforced policy rather than assume isolation cancels it. [Configuration basics](https://learn.chatgpt.com/docs/config-file/config-basic), [advanced configuration](https://learn.chatgpt.com/docs/config-file/config-advanced).

## Request contract and Ollama cross-check

The documented provider protocol is Responses, with separate WebSocket capability and provider-specific credentials. Model selection and an optional `model_catalog_json` are separate configuration concerns. Observed desktop requests used HTTP POST `/v1/responses`, `stream: true`, Bearer dummy authentication, and the selected Kimi ID; they also included tool definitions, instructions, reasoning, and client metadata. [Configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).

The pinned [Ollama Go launcher](https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/codex_app.go) was re-read alongside [the earlier source research](ollama-launchers.md). It teaches several concrete distinctions:

| Concern | Pinned Ollama implementation | Consequence for this launcher |
| --- | --- | --- |
| Discovery and mode | ChatGPT/Codex app paths, bundle `com.openai.codex`, `codex://threads/new?mode=codex` | Match identity and mode; app filename alone is insufficient. |
| Provider | Removes explicit root `model_provider`; points `openai_base_url` to its local router | A custom provider is a different, narrower connection. Copying only the URL loses routing behavior. |
| Catalog | Merges native and selected Ollama models; maintains a separate routing allowlist | Model-picker visibility does not decide upstream routing. |
| Credentials | Preserves existing login; can create an exact local sentinel if none exists | Do not copy this into a Token Factory auth mutation. The isolated test needed no sentinel auth file. |
| Native catalog discovery | Can copy auth/cache into a temporary Codex home | This experiment did not copy those files; that behavior is unnecessary for the bounded one-model route. |
| Lifecycle | `open`, restart consent, persistent root-config changes, recorded restore state | Desktop lifetime differs from waiting for a CLI child. Ordinary `open` can reuse a running instance. |
| Platforms | macOS and Windows branches | Windows source existence is not executed qualification. |

Ollama's [desktop router](https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/internal/proxy/codex_desktop.go#L185-L265) sends catalog-listed models to Ollama; other models retain OpenAI/ChatGPT routing. It rejects native requests carrying only its local sentinel instead of forwarding that token. It also has a separate auto-review resolver. This explains why its mixed-model design cannot be replaced by sending everything to Token Factory. The inspected dispatch does not generally rewrite arbitrary native model IDs into the selected model.

The fixture observed `gpt-5.6-luna` before the main Kimi request. The auxiliary request had structured-text settings and no tool definitions; the main request included tools. Its exact purpose was not established from those metadata alone. The bundle contains multiple auxiliary-generation paths, so calling it definitively “the title request” would exceed the evidence. A narrow launcher should explicitly reject unsupported model IDs and report auxiliary degradation, or establish a supported configuration for them; silent rewriting is not acceptable.

Ollama documents model requests and compaction routing plus a separate authenticated web-search service. Our two initial observed calls used only Responses; no complete desktop auxiliary-endpoint inventory has been demonstrated. Treat `/responses/compact`, model/catalog discovery, plugin/account services, auto-review, and optional search as separate test requirements, not automatically compatible Token Factory endpoints. [Ollama desktop contract](https://docs.ollama.com/integrations/chatgpt).

## Empirical results and remaining qualification

The first local fixture deliberately returned HTTP 503. Two metadata-only records were retained: one native auxiliary model and one Kimi request. The operator reported the exact visible local-fixture error and URL. Computer Use refused access to `com.openai.codex` for safety reasons; no alternative UI automation bypass was attempted, and the human operated the scratch window.

The follow-up local fixture streamed a synthetic function call, observed `function_call_output` containing `tofa-desktop-tool-ok`, and streamed its final text. The human saw “Offline tool round trip complete.” No additional user-turn request arrived in that phase, so offline continuation was not claimed.

### Live test through the existing launcher

The launcher executable reported `dev-prototype`; Go build metadata pinned the clean source to [`430531a86540f0a78d3814c85e04c6270efde9d7`](https://github.com/kreuzhofer/nebius-tofa-cli/tree/430531a86540f0a78d3814c85e04c6270efde9d7), SHA-256 `d8ecf7b18238c99a4c4719c5344151f7f97a2c67c308bf4e6e1e099ff4646e95`. This is a source-built evaluation executable, not a claim that a published release includes a desktop target.

A scratch shim occupied the launcher's `codex` executable slot. It received the existing adapter's endpoint/token and generated catalog arguments, wrote those settings only into a fresh desktop Codex home, and started the owned desktop process. The real Token Factory key remained inside the existing launcher adapter. A temporary diagnostic bridge allowed only the selected model, forwarded only `/responses`, limited output to 1,024 tokens per request, and recorded categorical metadata/SSE event counts. It performed no protocol/model translation. The existing adapter supplied its normal history repair. Token Factory documents the endpoint, Bearer auth, streaming, and output limit. [Nebius Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response).

| Local request | Result |
| --- | --- |
| 1: automatic title generation, now selecting Kimi with the launcher catalog | HTTP 400 from the existing adapter; not sent to Token Factory. Scratch desktop log identifies `ThreadMetadataGenerationService`. |
| 2: requested `exec_command` running `printf tofa-desktop-live-ok` | HTTP 200; streamed function-call argument events and completion; 10,423 input / 45 output tokens. |
| 3: tool-result continuation | Included one function-call output; HTTP 200; four output-text delta events and completion; 10,543 input / 21 output tokens. Human confirmed the command marker. |
| First extra user-turn attempt | Diagnostic HTTP 429 because the initial three-local-request budget included the rejected title request. This was not Token Factory rate limiting or model failure. |
| 4: same-thread user continuation after owned-instance restart | Included one tool-result and one assistant-history item; HTTP 200. Human received `tofa-desktop-live-ok`; the same scratch session recorded that assistant answer, `task_complete`, and 10,622 input / 13 output tokens. |

The limit was explicitly increased to four **cumulative local requests**, preserving the thread/state across restart. Three paid requests completed, totaling 31,588 input and 79 output tokens; those input totals include cached tokens. The observer logged an unclassified `transport_error` on request 4 and did not retain its final SSE summary. The independent client completion evidence is retained; the precise transport-error cause was not established.

The title error was: `Kimi-K3 tools with json_schema are unsupported except the recognized non-strict Codex approval review; request was not sent upstream`. The [adapter gate](https://github.com/kreuzhofer/nebius-tofa-cli/blob/430531a86540f0a78d3814c85e04c6270efde9d7/internal/tofa/adapter.go#L219) passes absent/empty tools and `tool_choice = "none"` unchanged; this failure is its unsupported tools-plus-schema branch, not ordinary empty-tools JSON generation. The diagnostic did not retain the exact title tool count or schema. Main tool execution and continuation succeeded despite that separately logged auxiliary failure. Do not silently claim title support or broaden the approval-specific workaround to arbitrary schemas.

### Restoration and validation

All owned desktop/app-server processes exited, all three diagnostic listener ports closed, and both experiment-owned app-state trees were removed. No ordinary app restart, credential copy, login/logout, or original configuration write was performed. The ordinary `config.toml` digest and filesystem metadata were unchanged between the pre-live-request baseline and final cleanup. Ordinary `auth.json` stat metadata was also unchanged; its contents were never read, so no byte-for-byte credential-store comparison is claimed. The comparison baseline was taken before the live requests, **not** before the earlier offline launches. No desktop scratch `auth.json` was created.

The experiment did not open or migrate existing conversations. That is preservation by separation, not a full history integrity audit. Enforced policy was not rewritten; conflict/managed-policy execution remains a qualification requirement. No new ordinary-account inference or independent ordinary-app functional test was performed after cleanup; the continuing host conversation is not treated as such a test.

`go test ./...` passed using the existing Go toolchain after rerunning with permission for localhost test listeners; the first sandbox-only attempt failed at `httptest` binding. Offline diagnostic-bridge checks passed for wrong credentials, nonselected-model rejection, request-count and output limits, and upstream credential replacement. JSON validation, local documentation links, and `git diff --check` were also checked. These checks do not certify other desktop modes or versions.
## Concrete launcher implementation scope

A follow-up should add an **experimental macOS Codex-mode target**, gated to a recorded app/engine combination, with the main-flow evidence above and the remaining limitations made explicit:

1. Discover the actual app bundle and bundled engine, reject incompatible versions, and show the product/mode explicitly.
2. Own separate Codex and Electron state, one verified model catalog, a provider credential channel, and an empty initial workspace. Do not alter the user's ordinary config, login, or history. Fail if effective state/routing differs from the planned instance.
3. Bind adapter lifetime to the owned desktop/app-server process, not the short-lived `open` command. Reject nonselected model IDs; surface auxiliary failures. Reuse the demonstrated existing Responses history repair. Resolve automatic-title generation narrowly: configure it without incompatible tools/schema if the desktop exposes a supported setting, or explicitly disable/report that feature. Do not generalize the approval-only adapter workaround.
4. Before exposing support, test streaming, real tool execution, tool-result continuation, another user turn, cancellation, authentication errors, unsupported models, catalog limits, auto-review, compaction, startup failure, normal exit, and interrupted cleanup. Verify ordinary app behavior afterward and exercise conflicting settings/managed policy.
5. Record macOS arm64 as the initial qualification boundary. Windows needs independent app discovery, credential delivery, state isolation, lifecycle, and model/tool testing; neither this run nor Ollama source supplies Windows acceptance.

Mixed native/Nebius account routing, global root-config replacement, broad protocol translation, and other desktop modes are separate work. [#9](https://github.com/kreuzhofer/nebius-tofa-cli/issues/9) remains deferred context and is not a blocker for this bounded finding.
