# Client configuration contracts for direct Token Factory inference

Research date: 2026-09-21. Ticket: [Establish supported configuration contracts for coding agents and desktop apps](https://github.com/kreuzhofer/nebius-tofa-cli/issues/3).

## Scope and evidence boundary

This investigation records client-side contracts for a standalone launcher. The current effort permits **direct Token Factory connections only**. Protocol translation and local proxies belong to a separate future effort. A configurable URL is not proof that the client, model, and endpoint interoperate.

Sources below are current vendor documentation, supplemented by Ollama's own integration documentation. No credentials were read, no paid inference was invoked, and no installed app configuration was changed. No cross-platform execution was performed. Documentation is mutable; explicit version boundaries below are documented introduction/change points, not a claim that a particular complete app release was tested. Ollama implementation details are covered by the separate launcher-source investigation.

## Compatibility matrix

| Surface | Required inference contract | Configuration mechanism | OS evidence | Direct Token Factory assessment |
| --- | --- | --- | --- | --- |
| Codex CLI | OpenAI **Responses**, including streaming and tool round trips | Custom provider, model ID, environment-sourced credential; CLI overrides/profile | Native macOS/Linux/Windows installation documented | Strong direct-prototype candidate: Nebius documents Responses. Full client/model behavior remains untested. |
| ChatGPT desktop, Codex mode | Ollama integration uses its Codex inference route and model catalog | Ollama manages desktop connection/profile and restarts app | ChatGPT itself: macOS/Windows/Linux; Ollama connector OS coverage needs source verification | No vendor-documented turnkey direct-Nebius desktop contract established here. Investigate separately from CLI. |
| ChatGPT desktop, ordinary Chat/voice | Native ChatGPT connection in the researched Ollama integration | No replacement mechanism established | Same app availability as above | No evidence supporting endpoint injection for these modes. |
| Claude Code CLI | Anthropic **Messages**, streaming, tool use | `ANTHROPIC_BASE_URL`, credential, explicit model; optional session settings | macOS/Linux/Windows | Technically configurable against a compatible endpoint. Anthropic explicitly does **not support non-Claude models through gateways**. |
| Claude Desktop, third-party mode | Anthropic **Messages** with streaming and tools | Dedicated third-party inference configuration; not CLI environment | Vendor documents macOS/Windows/Linux settings; Ollama advertises macOS, Windows forthcoming | Requires direct protocol compatibility plus application/model validation. Vendor contract describes Claude models, not a guarantee for arbitrary Nebius models. |

Evidence: [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference), [Codex CLI installation](https://learn.chatgpt.com/docs/codex/cli), [ChatGPT desktop](https://learn.chatgpt.com/docs/app), [Ollama ChatGPT integration](https://docs.ollama.com/integrations/chatgpt), [Claude Code gateway support boundary](https://code.claude.com/docs/en/llm-gateway), [Claude Code installation](https://code.claude.com/docs/en/setup), [Claude Desktop gateway contract](https://claude.com/docs/third-party/claude-desktop/gateway), [Claude Desktop configuration](https://claude.com/docs/third-party/claude-desktop/configuration), [Ollama Claude Desktop integration](https://docs.ollama.com/integrations/claude-desktop).

### Cross-check against Token Factory

Nebius documents `POST https://api.tokenfactory.nebius.com/v1/responses` with Bearer authentication and fields for streaming, tools, reasoning, `previous_response_id`, and `store`. Thus direct Codex research has a concrete protocol match to test, not merely a hypothetical future endpoint. [Nebius Responses API](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response).

Nebius's Claude Code cookbook instead uses a relay; it does not establish a direct Anthropic Messages endpoint. Under current scope, Claude integrations remain blocked on direct protocol evidence rather than silently adopting that relay. [Nebius Claude Code cookbook](https://dev.nebius.com/cookbook/claude-code-token-factory-relay).

Nebius also documents optional `ai_project_id` query scoping and model-list errors for missing/invalid project IDs. Codex exposes provider `query_params`; the proposed setting is `query_params = { ai_project_id = "<project-id>" }` where needed. This is a documented client mechanism, but propagation to every relevant request has not been tested. Launcher-owned model discovery must supply project scope independently. [Nebius model listing](https://docs.tokenfactory.nebius.com/api-reference/models/list-models), [Codex provider reference](https://learn.chatgpt.com/docs/config-file/config-reference).

## Codex CLI

### Provider and authentication

Relevant provider keys are `name`, `base_url`, `env_key`, `wire_api`, `requires_openai_auth`, `http_headers`, `env_http_headers`, `query_params`, and `supports_websockets`. `wire_api` currently accepts only `responses`; `requires_openai_auth` defaults to false. Use a separate provider ID and credential environment variable rather than sending a saved ChatGPT credential to the new endpoint. WebSocket support is a distinct capability, not implied by Responses support. [Configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).

A proposed configuration shape, **not a verified Nebius recipe**, is:

```toml
model = "<verified-token-factory-model-id>"
model_provider = "nebius-tofa"
web_search = "disabled"

[model_providers.nebius-tofa]
name = "Nebius Token Factory"
base_url = "https://api.tokenfactory.nebius.com/v1"
env_key = "NEBIUS_API_KEY"
wire_api = "responses"
requires_openai_auth = false
supports_websockets = false
# Add only when required for the selected account/project:
# query_params = { ai_project_id = "<project-id>" }
```

`NEBIUS_API_KEY` above is our proposed child-process variable name; it is not asserted to be a mandated Nebius variable. The base URL must produce the correct `/responses` URL after client path composition. Do not test this using a real secret until the destination is verified.

Codex supports arbitrary `-c key=value` overrides, TOML values, and `--model`. Custom providers cannot reuse `openai`, `ollama`, or `lmstudio`. Profiles changed in **0.134.0**: `--profile nebius-tofa` reads `~/.codex/nebius-tofa.config.toml`; legacy `[profiles.nebius-tofa]` and top-level `profile` no longer select that profile. `model_catalog_json` can be profile-specific. `CODEX_HOME` changes all Codex state, not only configuration. [Advanced configuration](https://learn.chatgpt.com/docs/config-file/config-advanced).

### Scope, precedence, restoration

Current precedence is CLI overrides, trusted project configuration, selected profile, user configuration, cloud defaults, system configuration, built-ins. User configuration is `~/.codex/config.toml`. Enforced organization requirements are separate from defaults. [Configuration basics](https://learn.chatgpt.com/docs/config-file/config-basic).

Provider/authentication keys are specifically excluded from project configuration despite the general precedence order. Therefore a repository-local `.codex/config.toml` is not the correct place for endpoint switching. For a first prototype, prefer explicit process arguments and environment; alternatively own one named profile. Remove only launcher-owned files on disconnect. Do not rewrite `auth.json`, erase login state, or assume changing `CODEX_HOME` preserves users' skills and history. These are design recommendations derived from the documented layers. [Advanced configuration](https://learn.chatgpt.com/docs/config-file/config-advanced).

### Model catalog and protocol proof

Ollama refreshes a generated catalog, uses a dedicated launch profile, and provides `--config` and `--restore`. Its example points to `model_catalog_json` and a provider with `wire_api="responses"`. It recommends at least 64K context. `codex --oss` is a distinct local-provider route; it is not a general third-party cloud switch. [Ollama Codex CLI integration](https://docs.ollama.com/integrations/codex).

The complete catalog schema and capability values accepted by each Codex version were not established by the vendor pages retrieved here. Do not invent entries or copy Ollama's capabilities onto unrelated models. Initial explicit model selection can avoid promising a full catalog UI until its schema is pinned.

Responses tool use includes function-call output items linked by `call_id`, and models may emit multiple tool calls. Some models emit custom-tool calls or reasoning items as well. [OpenAI function calling](https://developers.openai.com/api/docs/guides/function-calling). Streaming uses typed SSE events, not Chat Completions' `choices[].delta` shape. [OpenAI streaming responses](https://developers.openai.com/api/docs/guides/streaming-responses). **Inference:** matching a base URL and accepting one text prompt is insufficient; direct support must include the requests this Codex/model combination actually sends.

## ChatGPT desktop

Follow-up: [macOS Codex desktop feasibility](codex-desktop-feasibility.md) pins the installed ChatGPT/Codex product and bundled engine, tests a separate desktop instance, and records routing and qualification evidence. The original documentation-only boundaries below remain historical findings.

OpenAI's current desktop page lists macOS, Windows, and Linux and distinguishes ChatGPT Chat/Work from Codex. Its ordinary onboarding uses a ChatGPT account. This is evidence of app availability, not proof of custom-provider entitlement or the same integration implementation on all systems. [ChatGPT desktop documentation](https://learn.chatgpt.com/docs/app).

Ollama's documented integration requires **Ollama 0.34.0+**, adds up to five compatible models to the Codex picker, and restarts ChatGPT when needed. Native model requests continue to their native provider. Ordinary Chat and voice are explicitly outside Ollama routing. Codex keeps session ownership; Ollama handles selected-model requests and compaction. Disconnect restores connection settings and the previous profile. Ollama web search additionally requires Ollama sign-in. [Ollama ChatGPT integration](https://docs.ollama.com/integrations/chatgpt).

**Unresolved:** exact vendor-supported desktop launch flags, connection settings, catalog schema, OS-specific paths, account requirements for third-party Codex-mode models, and whether the desktop relies on auxiliary endpoints absent from direct Token Factory. Ollama's working integration is evidence to study, not permission to infer that an environment variable redirects the entire desktop app. Do not include ordinary Chat, voice, or unrestricted Work-mode replacement in the supported-client list based on this evidence.

## Claude Code CLI

### Authentication and one-session injection

`ANTHROPIC_BASE_URL` chooses the endpoint; it does not change the model. `ANTHROPIC_AUTH_TOKEN` sends `Authorization: Bearer`, whereas `ANTHROPIC_API_KEY` sends `x-api-key`. `apiKeyHelper` sends both. A gateway credential supersedes a saved login; removing it restores that login. A base URL alone leaves the saved subscription credential active. Settings-file `env` values override shell exports. Desktop launchers from the dock/start menu do not reliably inherit shell exports. [Gateway connection documentation](https://code.claude.com/docs/en/llm-gateway-connect).

A proposed direct-launch contract must therefore specify all three values:

```text
ANTHROPIC_BASE_URL=<verified-Anthropic-compatible-base-URL>
ANTHROPIC_AUTH_TOKEN=<Token-Factory-key-if-Bearer-is-accepted>
claude --model <verified-model-id>
```

This is a conditional contract, not a tested Nebius command. Do not set both key variables indiscriminately. Inspect conflicting provider selection such as Bedrock/Vertex/Foundry before launch; user or managed routing must not silently win while the UI claims Nebius.

`--settings <regular-json-file-or-inline-json>` supplies session overrides and merges omitted values from existing settings. `--setting-sources` controls loading user/project/local settings. [CLI reference](https://code.claude.com/docs/en/cli-reference). Managed settings outrank CLI settings, then project-local, project-shared, and user settings. Use a temporary restricted-permission settings file for sensitive overrides rather than secrets in process arguments; preserve policy and unrelated settings. Delete it after the owned session ends, accounting for background workers. That file-handling recommendation is ours. [Settings precedence](https://code.claude.com/docs/en/settings).

### Model selection and capability pitfalls

`--model` overrides `ANTHROPIC_MODEL`, which overrides the `model` setting. Relevant aliases use `ANTHROPIC_DEFAULT_OPUS_MODEL`, `ANTHROPIC_DEFAULT_SONNET_MODEL`, `ANTHROPIC_DEFAULT_HAIKU_MODEL`, and now `ANTHROPIC_DEFAULT_FABLE_MODEL`; `CLAUDE_CODE_SUBAGENT_MODEL` controls otherwise-unspecified subagents. `ANTHROPIC_SMALL_FAST_MODEL` is deprecated. A custom picker entry can use `ANTHROPIC_CUSTOM_MODEL_OPTION` with optional `_NAME` and `_DESCRIPTION`. `CLAUDE_CODE_MAX_CONTEXT_TOKENS` can correct unrecognized custom IDs; exact applicability depends on the ID and version. Preserve a real model ID rather than disguising it as a Claude family. `/model` can persist user defaults, so launch-time flags are more reversible. Managed model allowlists remain effective. [Model configuration](https://code.claude.com/docs/en/model-config).

Gateway discovery is opt-in through `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1`. Do not assume a Token Factory OpenAI-style model listing is automatically accepted. Before **2.1.246**, gateway credentials could accompany some Anthropic telemetry/usage requests; current docs document that correction and `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`. Remote Control and voice have separate login restrictions; gateway authentication is not feature-equivalent to subscription login. [Gateway connection documentation](https://code.claude.com/docs/en/llm-gateway-connect).

### Protocol and support boundary

The documented Anthropic route posts to `/v1/messages?beta=true`, streams responses, and may call optional `/v1/messages/count_tokens`; absence of counting falls back to estimation. It sends `anthropic-version` and evolving `anthropic-beta` headers. Unknown model IDs may receive adaptive-thinking/effort/context-management assumptions. Streaming keep-alives matter. A `HEAD /api/hello` startup probe is best-effort. These are direct-endpoint compatibility checks, not instructions to build an adapter. [Gateway protocol](https://code.claude.com/docs/en/llm-gateway-protocol).

Most importantly, Anthropic documents the gateway mechanism while explicitly declining support for routing Claude Code to **non-Claude models**. Therefore “Anthropic-compatible” and “vendor-supported Claude Code on Nebius models” are different claims. The latter is not established, even if an empirical prototype succeeds. [Gateway support boundary](https://code.claude.com/docs/en/llm-gateway).

Native installs cover macOS 13+, Windows 10 1809+/Server 2019+, and listed Linux distributions on x64/ARM64. Git for Windows is recommended; PowerShell is the documented fallback shell. Native installation does not require the user to provision Node; npm is an alternative distribution. [Advanced setup](https://code.claude.com/docs/en/setup).

## Claude Desktop

Third-party mode is a separate deployment mode covering **Chat, Cowork, and Code**, with local conversation storage and configured-provider inference. It can offer startup without Anthropic authentication when provider credentials exist. This does not imply that existing first-party cloud conversations or every account feature transfer unchanged. [Third-party overview](https://claude.com/docs/third-party/claude-desktop/overview).

The documented inference keys are:

```json
{
  "inferenceProvider": "gateway",
  "inferenceCredentialKind": "static",
  "inferenceGatewayBaseUrl": "<verified-Messages-base-URL>",
  "inferenceGatewayApiKey": "<credential>",
  "inferenceGatewayAuthScheme": "bearer"
}
```

Messages streaming and tool use are required; model discovery via `/v1/models` is optional. `inferenceModels` can provide explicit IDs. Discovery normally filters for recognizable Claude models, so unrelated model IDs are not guaranteed to appear. The docs describe a Claude-model gateway, not general model support. Base URL/key date to **1.2581.0**, auth scheme to **1.3036.0**. No verified Nebius desktop recipe follows merely from these keys. [Gateway configuration](https://claude.com/docs/third-party/claude-desktop/gateway).

The supported local workflow enables Developer Mode, opens Configure Third-Party Inference, applies changes, and relaunches. Managed configurations make this form read-only. JSON export exists. There is no established session-only CLI switch equivalent to `claude --settings` here. [In-app configuration](https://claude.com/docs/third-party/claude-desktop/in-app-configuration).

| OS | Documented local configuration directory |
| --- | --- |
| macOS | `~/Library/Application Support/Claude-3p/configLibrary/` |
| Windows | `%LOCALAPPDATA%\Claude-3p\configLibrary\` |
| Linux | `~/.config/Claude-3p/configLibrary/` |

Linux managed configuration is `/etc/claude-desktop/managed-settings.json`, with ownership/permission checks. `inferenceProvider` activates third-party mode; `inferenceCredentialKind` selects one source without fallback. The docs distinguish these settings from standard desktop configuration, so changing MCP configuration is not the endpoint-switching contract. [Configuration reference](https://claude.com/docs/third-party/claude-desktop/configuration).

macOS managed preferences outrank local configuration. Windows machine policy at `HKLM\SOFTWARE\Policies\Claude` takes precedence over user policy; since **1.19367.0**, presence of machine policy prevents reading user policy altogether. The app may download required runtime components from `downloads.claude.ai`. Thus launcher runtime independence does not promise that target applications need no downloads. [MDM deployment](https://claude.com/docs/third-party/claude-desktop/mdm).

Ollama's desktop integration documents macOS support, future Windows support, previous-settings restoration, and `ollama launch claude-desktop --restore`; quitting Ollama also restores settings. Its supported extras include Cowork and Ollama web search. Those are Ollama-specific behaviors, not guarantees of a direct Token Factory route. [Ollama Claude Desktop integration](https://docs.ollama.com/integrations/claude-desktop).

## What must be settled before the first prototype claims support

These are research-derived acceptance criteria, not completed tests:

1. Select an exact client version and direct endpoint/model combination; record protocol, credential header, URL path composition, and supported context length.
2. Demonstrate streaming text, a tool call with structured arguments, tool-result continuation, cancellation, and understandable authentication/model errors. A plain text response alone is insufficient evidence.
3. Verify effective routing with conflicting user/project settings present. Preserve managed policy. Do not report Nebius active when another configuration wins.
4. Exercise launch, normal exit, interruption, and subsequent ordinary client launch without losing login, history, or user configuration.
5. Treat CLI and desktop as separate integration adapters. A standalone launcher can support all three operating systems while some application integrations remain unavailable or experimental.
6. Distinguish **documented configuration**, **provider-supported protocol**, **empirically tested behavior**, and **vendor-supported model use** in the support table.

Unresolved protocol gaps remain blockers under the current direct-only scope. This report does not recommend silently adding a proxy, impersonating Claude model capabilities, or promising broad desktop compatibility to hide those gaps.
