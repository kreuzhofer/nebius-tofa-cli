# Installed desktop title source refresh for #41

Read-only refresh on 2026-09-29 for [#41](https://github.com/kreuzhofer/tofa-launcher/issues/41).
The candidate remains exactly `nvidia/Nemotron-3_5-Lightning`. This supplements
[the original investigation](lightning-chat-naming.md) and
[the subsequent Kimi qualification](desktop-shared-title-generation.md); it does
not repeat a paid campaign or qualify Lightning.

The installed desktop still selects a service-configured auxiliary model, whose
source default is `gpt-5.6-luna`, for titles. Its fresh-thread helper permits
provider model fallback. Neither property supplies an independent, user-configured
Lightning naming role. With Luna retained in the authoritative catalog, changing
catalog priority alone does not activate fallback for that request. The pinned
engine source, historical bundled-engine replay and live #52 observation establish
this retention behavior; the source refresh below establishes that the desktop members have not changed
since that investigation. [Service and helper sources](#primary-source-locators),
[#52 reproduction](desktop-shared-title-generation.md#reproduction-and-correction).

The installed engine's version tag `rust-v0.155.0-alpha.16.4` resolves through
annotated tag `cede03d39bfee42f29e92e20cd5d21cedf5a5d90` to commit
`3853cf0c49daadcacaacceb2cbb732f512eaacdb`. In
[`StaticModelsManager::get_default_model`](https://github.com/openai/codex/blob/3853cf0c49daadcacaacceb2cbb732f512eaacdb/codex-rs/models-manager/src/manager.rs#L593),
an explicit requested model is preserved if present in the available catalog,
including when fallback is enabled. With fallback enabled, an unavailable requested
model or no explicit model falls through to the catalog default. This version-matched
source supports the selection inference without claiming a fresh runtime capture.

## Exact inspected installation

| Component | Identity read locally |
| --- | --- |
| Application | `/Applications/ChatGPT.app`, bundle `com.openai.codex`, version `26.917.71314`, build `10954`, from `Contents/Info.plist` |
| Bundled engine | `Contents/Resources/codex --version`: `codex-cli 0.155.0-alpha.16.4` |
| Engine SHA-256 | `93169e745735930598e867ad837abf3fdc50774a3ad7e7aa89c0d0c51b0189a5` |
| `Contents/Resources/app.asar` SHA-256 | `03108a728bdb1616958ab89587c5495cab0cf4cd1bbe109bdfb186df0a113804` |
| Launcher source baseline | [`c5e68e0f35d3086bdbce80ba1f43721028751c30`](https://github.com/kreuzhofer/tofa-launcher/tree/c5e68e0f35d3086bdbce80ba1f43721028751c30) |

These are direct bundle/file observations, not an upstream reproducible-build
attestation. Only version output was requested from the engine. The application
was not launched, and no user configuration, credential, conversation database or
remote routing-service state was read. No authenticated availability or current
pricing claim follows from this work.

## Primary source locators

The following first-party implementation members were extracted directly from
the installed ASAR without executing their JavaScript. Their hashes match the
members recorded by #52. Locators are exact searchable names/strings within those
members; minified line numbers are unsuitable. Claims referring to these locators
are source findings, not observations of a new desktop session.

| Archive member | SHA-256 | Locators |
| --- | --- | --- |
| `.vite/build/main-C-Mhak1n.js` | `457c79be69620d4489e94c14ac665f81731d869635606dcf175b7b4cc2e8b467` | `var I5=class`, `ThreadMetadataGenerationService`, `async generateTitle`, `async generateDescription`, `async generateSummary`, `async function Ln`, `codex-app-model-routing` |
| `.vite/build/src-DldfpmrL.js` | `88ec69722b5d87a7081edf2e2d6a2e21c3f25300cee587363d74b8b87969a412` | `P_`, `Hte`, `Ute`, `Wce`, `Gce`, `Kce`, `X9`, `L8`, `TN`, `EN`, `G9`, `Bce`, `V9`, `Jne`, `Y9`, `qce`, `kN`, `LN`, `R8`, `Aie`, `Mie` |
| `webview/assets/app-initial-51da50e6c6e3.js` | `ea4b3893669a56e67f6baa7d2791ed64e954b081a549f62b647101def36ef6d1` | `sDn`, `lDn`, `uDn`, `dDn`, `threadMetadataGeneration` |

To reproduce member extraction: read the ASAR's little-endian header size at byte
4, JSON length at byte 12, and JSON header beginning at byte 16. The data section
starts at `8 + header_size`. Traverse the header's nested `files` map with the
member path, then read `size` bytes at `data_start + int(offset)`. SHA-256 the raw
member bytes before decoding them for text searches. This requires no installed
package, account or app launch.

## Selection and title contract

`I5.generateTitle` obtains `ephemeralGenerationModel` from `Ln`, which reads the
`codex-app-model-routing` dynamic service configuration. `P_`, `Hte` and `Ute`
define the `gpt-5.6-luna` default and the service field
`codex_ephemeral_generation_model_slug`. No model argument is exposed in the
service's `generateTitle` parameter list. This is a remote service configuration
path, not evidence of a user `config.toml` title selector. The actual current
remote value was not queried. [Main `I5`, `Ln`; source `P_`, `Hte`, `Ute`](#primary-source-locators).

The ordinary service calls `Wce → X9 → L8 → TN`. `Wce` identifies the operation as
`thread_title`; `X9` uses effort `low`; `L8` starts an ephemeral thread with
`modelProvider: null`, `allowProviderModelFallback: true`, `approvalPolicy: never`,
read-only permissions and empty runtime workspace roots. The normal title service
does not supply a source thread. `TN` then uses `model: null` to inherit the
temporary thread selection and sends `turnTrigger: thread_title` plus the output
schema. This does not rewrite the requested model to Lightning. [Source `Wce`,
`X9`, `L8`, `TN`](#primary-source-locators).

`G9` requires a title string of 1–36 characters and a nonempty description string;
`Bce` derives the output schema. `EN` parses JSON and validates it with the response
schema before `Jne` normalizes the title and `Y9` normalizes/truncates the description
to 100 characters. This is the desktop title-and-description contract, not the
CLI title-only contract. The historical wire fixture records strict
`codex_output_schema` serialization; extraction of these members is not a new wire
capture. [Source `G9`, `Bce`, `EN`, `Jne`, `Y9`](#primary-source-locators),
[retained reduced wire fixture](../../internal/tofa/testdata/desktop-native-title-request.json).

The helper disables fanout, hooks, multi-agent, plugins, shell snapshots, tool
suggestions and web search, and disables its `codex_app` MCP server. `qce` allows
only explicitly supplied read-only app tools. It does not explicitly disable shell,
unified execution or code mode. Read-only permissions therefore do not establish
an empty model-visible tool inventory. #52 captured code-mode definitions inside
an `additional_tools` input item, demonstrating why an absent top-level `tools`
field alone is insufficient. [Source `X9`, `qce`, `L8`](#primary-source-locators),
[#52 request-shape observation](desktop-shared-title-generation.md#reproduction-and-correction).

## Deadline, cancellation and persistence limits

`V9` is 30,000 ms, but the timer is installed inside `TN`, **after** `L8` awaits
thread creation or forking. It bounds structured-result waiting, not the whole
title operation including dynamic-config lookup, app lookup or thread startup.
On timeout or abort, `TN` removes its notification handler and rejects; it attempts
`interruptTurn` only when the client supports it and a turn ID is known. If a late
`startTurn` result supplies that ID after timeout, its continuation attempts the
interrupt. `L8` finally attempts to unsubscribe and ignores unsubscribe errors.
These are best-effort cleanup mechanisms, not proof that an upstream request or
billing necessarily stops at 30 seconds. [Source `V9`, `L8`, `TN`](#primary-source-locators).

The ordinary `generateTitle` service supplies no abort signal; summaries do have
abort controllers for superseded requests and service disposal. Renderer `lDn`
avoids replacing an existing nonmatching title, installs a provisional first-input
title, and conditionally persists a generated title only while the expected title
still matches. Null/error generation persists the first-input fallback through
`uDn`. A visible provisional title is therefore not proof of successful inference.
These are inspected guards; no new UI persistence test ran for this refresh.
[Main `I5`; renderer `lDn`, `uDn`](#primary-source-locators).

## Generic fallback affects more than titles

| Path | Source-backed consequence of changing a provider default |
| --- | --- |
| Titles and summaries | `Wce` and `Kce` both call `X9 → L8`, whose fresh-thread path enables fallback by default. A missing requested auxiliary model could redirect both. |
| Descriptions | `Gce` shares the requested auxiliary model but passes a source thread and `fallbackToFreshThread: false`. `L8`'s fork request does not send the fresh-thread fallback flag. Do not assume identical fallback behavior for successful forks. |
| Ambient suggestions and safety classification | Their separate `kN` helper starts fresh ephemeral threads with fallback enabled. `LN` uses source `ambient_suggestion_safety`; the suggestion path uses configured suggestion models, and first-plugin-connect can use `ephemeralGenerationModel`. These are outside title-only routing. |
| Pull-request generation | `Aie` passes a provider-fallback capability check to `R8 → L8`; the effect depends on advertised engine support and whether its requested model is present. |
| Generic text generation | `Mie` explicitly passes `allowProviderModelFallback: false`; sharing `R8` does not imply every caller enables fallback. |

This is a bounded source inventory, not proof that every feature is enabled or
executed in the user's account. Each row follows the named members in the
[primary source table](#primary-source-locators).

## Practical follow-up

The former “put Lightning first” lead needs a stricter prerequisite: the requested
auxiliary model must be absent before provider fallback can choose a different
default. #52 observed native Luna preserved in the merged catalog and retained by
the engine despite fallback being enabled. Dropping or relabeling native catalog
entries would change the established preservation contract and could affect the
other operations above. Treat that as new product work, not a configuration recipe.
[#52 retention evidence](desktop-shared-title-generation.md#reproduction-and-correction),
[desktop route contract](../codex-desktop.md).

The realistic next step is an upstream title-model/provider selector, or a separately
specified, explicit launcher naming policy extending the existing recognized title
route. The latter would need exact Lightning metadata, a captured request contract,
visible role routing and bounded qualification of schema validity, streaming,
latency, failure handling and title persistence. Existing Kimi title qualification
does not transfer to Lightning, and a successful synthetic engine boundary replay
would establish selection/serialization only. Preserve main and Guardian selection,
normal settings, strict request validation and native catalog descriptors; do not
patch the installed app or silently substitute model IDs. [Existing Kimi scope and
limits](desktop-shared-title-generation.md), [original bounded qualification
proposal](lightning-chat-naming.md#narrow-next-work-and-bounded-qualification),
[main identity ADR](../adr/0001-preserve-conversation-model-identity.md),
[Guardian route ADR](../adr/0002-guardian-selection-follows-launch-route.md).

Before paid qualification, an explicitly owned isolated desktop session can use a
loopback provider and synthetic input to delay the main response while capturing
title thread creation, requested/resolved model IDs, both title markers and exact
serialization. Complete main and title responses separately, then exercise title
timeout/failure and manual-title preservation. This would distinguish independent
title dispatch from a title inferred only after main completion, and would measure
the actual UI lifecycle around the source's narrower timer. This delayed-main/UI
experiment remains a proposal. The initial installed-engine/desktop checks were
blocked by the incumbent-desktop ownership guard. After the maintainer closed the
app, all six previously blocked checks and the CLI title non-routing control passed
in 39.843 seconds. The installed-engine replay retained each requested Luna model
and routed its recognized title request to the synthetic Kimi upstream. This
replayed public engine calls using a fixture desktop executable, not automatic
Electron title dispatch. The guard was not bypassed. See the [rerun evidence and
exact test names](lightning-chat-naming.md#checks-performed-and-unavailable-coverage).

No Lightning inference or compatibility test was run for this source artifact.
Verification consisted of current bundle metadata, engine version, fresh hashes,
direct member extraction and inspection of the cited source paths. Main-agent
portable and installed-engine boundary-test results are recorded separately in the
accompanying issue continuation; they do not turn source inspection into UI or
Lightning qualification.
