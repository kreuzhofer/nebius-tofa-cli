# Kimi-K3: public Token Factory metadata

Investigated **2026-09-21**, final provider-source check **13:04 UTC**, for [Establish truthful Codex metadata for Token Factory Kimi-K3](https://github.com/kreuzhofer/nebius-tofa-cli/issues/14). Public documentation and unauthenticated catalog GETs only; no credentials or remote inference. Codex catalog/schema checks were performed separately by the parent investigation; see [the existing compatibility research](codex-kimi-followup.md).

## Conclusion

**Provider verification is now available for context, modalities and broad capabilities.** Nebius's public catalog explicitly identifies its JSON endpoint as authoritative. For `moonshotai/Kimi-K3`, that endpoint advertises **`max_model_len: 1024000`**, text/image input with text output, function calling, reasoning and Responses API support. This closes the earlier provider-evidence gap; use the provider's **1,024,000**, not the author's **1,048,576**, when representing the advertised deployment limit. These are catalog claims, not runtime boundary tests. [Catalog guide][catalog] [Provider JSON][provider]

This is enough to ground a **limited, truthful model descriptor** with a provider-advertised context budget. It is **not** enough to assert a complete Kimi Responses reasoning contract, an output-token ceiling, or a supported direct Codex connection. The parent's fixture proves warning removal is technically possible; a usable descriptor must also preserve coding instructions and explicitly distinguish launcher policy from unknown capabilities. Do not invent missing values just to satisfy the catalog schema. Removing the warning does not resolve the separate continuation failure documented in the existing research.

## Client mechanics: parent-supplied evidence

The parent reports a one-request scratch-HOME/loopback reproduction with installed **Codex 0.155.1**, pinned to [`rust-v0.155.1`, commit `be2951ea34f0d295ed0becf97079f92fa5f6950e`](https://github.com/openai/codex/tree/be2951ea34f0d295ed0becf97079f92fa5f6950e). Selecting `moonshotai/Kimi-K3` prints “Model metadata for `moonshotai/Kimi-K3` not found. Defaulting to fallback metadata; this can degrade performance and cause issues.” and the diagnostic exits 1. A 32,768-token context override still fails; synthetic `gpt-5.4` and exact-slug `model_catalog_json` controls pass. No real key or inference was used.

The parent's source inspection finds [longest-prefix matching followed by matching after stripping one namespace prefix](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/models-manager/src/manager.rs#L690), then fallback on absence. Optional context/max-context fields left unset produce [no resolved context or derived autocompaction threshold](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/protocol/src/openai_models.rs#L504). A minimal catalog does not inherit local `model_messages`; [missing instruction templates yield empty instructions](https://github.com/openai/codex/blob/be2951ea34f0d295ed0becf97079f92fa5f6950e/codex-rs/protocol/src/openai_models.rs#L530). Consequently the synthetic catalog is **diagnostic only**, not production metadata. These client findings were supplied by the parent, not independently reproduced by the provider researcher.

The standalone [metadata diagnostic](../../scripts/repro_codex_metadata.py) accompanies this note on `research/kimi-codex-metadata`. Run `python3 scripts/repro_codex_metadata.py`: default exit 1 means the warning reproduced; `--model gpt-5.4` is the synthetic control (exit 0); `--context-window 32768` still exits 1; `--catalog <fixture.json>` accepts a diagnostic catalog fixture. Credentials and endpoint are synthetic throughout. The parent handles review and publication of both files.

## Evidence table

| Property | Positive evidence and scope | Remaining gap / implementation consequence |
| --- | --- | --- |
| Identity and availability | **Provider:** catalog model `Kimi-K3`, vendor `moonshotai`, status `active`; flavor ID `moonshotai/Kimi-K3`. [Provider JSON][provider] | Public availability does not establish access for a particular project or target-client compatibility. |
| Context | **Provider:** flavor `max_model_len = 1024000`; model and flavor `context_window_k = 1024`. **Author:** model card specifies `1048576`. [Provider JSON][provider] [Pinned model card][author] | Preserve the provider's explicit integer; do not multiply the abbreviated K field by 1024. Exact enforcement, prompt/output budgeting and tokenizer overhead remain untested. |
| Output limit | **Provider schema:** Responses `max_output_tokens` includes visible output and reasoning; no numeric ceiling/default is specified there. The Kimi catalog record has no separate output limit. [OpenAPI][schema] [Provider JSON][provider] | Unknown deployed maximum. Chat Completions documentation's default `max_tokens = 8192` is neither a Kimi maximum nor a Responses default. The author's `max_tokens=4096` example is also not a limit. |
| Modalities | **Provider:** `type` and flavor `model_type` are `image2text`; `use_cases` includes `text` and `image`. **Author:** model summary lists Text, Image. [Provider JSON][provider] [Pinned model card][author] | Supports advertising text/image inputs and text output. No Kimi-specific image count, resolution, token accounting or supported Responses image-detail settings were found. Do not infer image generation. |
| Tools and API | **Provider:** `use_cases` contains `function_calling`, `reasoning`, `responses_api`. Generic tool documentation describes client-executed functions. [Provider JSON][provider] [Function calling guide][tools] | Broad function-calling support is positive evidence. Parallel calls, strict schemas, custom/freeform tools, native shell/apply-patch tools and Codex tool-history round trips are not established by these tags. |
| Reasoning effort | **Author:** thinking always enabled; Chat Completions uses top-level `reasoning_effort`, supporting `low`, `high`, `max`, default `max`. **Provider schema:** Chat Completions includes `max`; nested Responses `reasoning.effort` lists `none`, `minimal`, `low`, `medium`, `high`, `xhigh` and null, **not `max`**. [Pinned model card][author] [OpenAPI][schema] | No published Kimi Responses accepted-value/default/mapping contract found. Shared `low`/`high` spellings do not prove model-specific handling. Never assume `xhigh = max`, or that `none` disables Kimi thinking. |
| Reasoning summary / verbosity | **Provider schema:** `reasoning.summary` and `generate_summary` enumerate `auto`, `concise`, `detailed`; `text.verbosity` enumerates `low`, `medium`, `high`. [OpenAPI][schema] | Generic schema presence does not establish Kimi behavior. Its `Reasoning` description still refers to GPT-5/o-series. Unknown whether Kimi accepts, ignores or implements these settings, and whether summaries preserve full thinking. |
| History | **Author:** replay complete assistant messages, including `reasoning_content` and `tool_calls`. **Provider schema:** accepts reasoning and function-call history items and documents replay of reasoning items. [Pinned model card][author] [OpenAPI][schema] | No Kimi-specific mapping of author thinking history to Responses plaintext/summary/encrypted items, or stateful continuation guarantee, was found. See precise requirements below. |

## What the public schemas establish

Nebius publishes a richer catalog route than a basic OpenAI-style model list: [`GET /v1/models?verbose=true` documentation][models] exposes `RichModel.context_length`, `architecture.modality`, nullable `per_request_limits`, `supported_features` and `supported_sampling_parameters`. That endpoint requires bearer authentication, so it was **not called**. The separately published `/api/public/models_info` requires no credentials and supplied the Kimi facts above. Its Kimi record contains no effort enumeration, reasoning-summary policy or output cap; a schema field's existence cannot supply a missing record value.

The pinned OpenAPI's `CreateResponseRequest.input` union includes these relevant branches:

- `ResponseOutputMessage` requires `id`, `content`, `role`, `status`, `type`; nested `ResponseOutputText` requires `annotations`, `text`, `type`.
- `ResponseReasoningItem` requires `id`, `summary`, `type`; optional `content` carries `reasoning_text`, while `encrypted_content` and `status` are optional. Its description instructs callers to replay reasoning items on subsequent turns.
- `ResponseFunctionToolCall` requires `arguments`, `call_id`, `name`, `type`; `FunctionCallOutput` requires `call_id`, `output`, `type`.

`previous_response_id`, `store` and `include: ["reasoning.encrypted_content"]` exist in the generic request schema. This establishes their published shapes, not that Kimi implements storage, encryption or preserved thinking through them. Likewise, `additionalProperties: true` does not remove the nested effort enum constraint or document a supported alternate Kimi parameter. No request was sent to test validation or behavior. [OpenAPI][schema]

## Engineering inquiry

Ask Nebius to publish a versioned contract for **`moonshotai/Kimi-K3` at `https://api.tokenfactory.nebius.com/v1/responses`**:

1. Confirm `max_model_len=1024000` as the enforced combined context budget, document input overhead, and give the maximum/default `max_output_tokens`, including reasoning accounting and behavior when input plus reserved output exceeds context.
2. List accepted Kimi `reasoning.effort` values and the omitted/null default. Explain the Chat Completions `max` versus Responses enum discrepancy: is `max` unsupported, omitted accidentally, or mapped to another value? Are unsupported values rejected or ignored?
3. Specify whether `summary`, `generate_summary` and `text.verbosity` affect Kimi. Identify the emitted reasoning fields/events and whether summaries are full replayable reasoning or lossy display text.
4. Supply redacted two-turn and tool-round-trip **Responses JSON/SSE examples** showing exactly which assistant, reasoning and tool items must be replayed, their IDs/status/annotations, and how they retain Moonshot's required complete thinking history. Clarify `store=false`, `previous_response_id` and encrypted-content behavior.
5. Confirm Responses image limits and tool capabilities separately: function tools, parallel calls, strict arguments, custom/freeform tools, shell and apply-patch. Publish these distinctions in the public catalog or linked documentation.

For the launcher, display name, visibility, priority, shell exposure, tool-output truncation and compaction headroom are **launcher policy**, not provider measurements. Disabling an unverified optional request feature can be a deliberate policy; it must not be presented as proof that the model lacks it. The parent should check whether the current Codex catalog can express these distinctions before supplying a descriptor. Continued-conversation compatibility remains a separate acceptance gate.

## Source pins and search boundary

All sources retrieved 2026-09-21. SHA-256 values hash fetched raw response bytes unless stated otherwise; mutable catalog pages supplied no immutable revision or Last-Modified header.

| Source | Revision / snapshot |
| --- | --- |
| [Authoritative public JSON][provider] | SHA-256 `d00c0eb7718afc9124118d0afdbe53d153c1f09e9bee39ea679c2823eb6d2b95`. Kimi record alone: `ddfcc43c2156e3feaa6c0fac598dd146a813076c105cafa8eddfdca650f2619d`, using Python `json.dumps(record, sort_keys=True, separators=(',', ':'), ensure_ascii=False)` encoded as UTF-8. |
| [Public catalog Markdown][catalog] | SHA-256 `a5fc970b2233ecf9525897d183ef9af889b0e804b21c39c142337066b8b39e64`; explicitly designates the JSON source authoritative. Both are linked from the public Token Factory homepage. |
| [Nebius OpenAPI][schema] | `info.version = 20260916-b712a99a4`; SHA-256 `e6ba0e6dc303e48fb8eec5d62fcc29cfc2f552a7725a2ae0903719023961ddb5`. Documentation snapshot, not a deployed-server revision. |
| [Moonshot model card][author] | Repository revision `f831ab66814297da540d832a5235f8e904f29d06`; [Hugging Face metadata](https://huggingface.co/api/models/moonshotai/Kimi-K3) reports last modified `2026-09-02T02:22:41.000Z`. Raw README SHA-256 `57de265b5842dfa465c6e73b368b0e15a89b8793b5450528dad577da202cc6fe`. |

Also checked the [Nebius documentation index](https://docs.tokenfactory.nebius.com/llms.txt), inference overview, function-calling guide, serverless guide, model-list reference, and Moonshot's [K3 quickstart](https://platform.kimi.ai/docs/guide/kimi-k3-quickstart). Nebius's [OpenCode cookbook](https://dev.nebius.com/cookbook/opencode-nebius-token-factory) independently lists Kimi-K3 at “1M” context, checked there on 2026-08-28; its [OpenHands cookbook](https://dev.nebius.com/cookbook/openhands-agent-canvas), dated by its verification on 2026-09-05, recommends Kimi-K3 for tool use. Neither fills the missing Responses contract. No authenticated catalog, private documentation or current Codex source internals were inspected. “Unknown” here means absent from this bounded public evidence, not unsupported by the service.

Validation: re-fetched the public catalog and checked its hashes; compared table values to the selected model/flavor and parsed OpenAPI definitions. Documentation-only change; runtime tests were not run because credentials and inference were expressly excluded.

[provider]: https://tokenfactory.nebius.com/api/public/models_info
[catalog]: https://tokenfactory.nebius.com/model-catalog.md
[schema]: https://api.tokenfactory.nebius.com/openapi.json
[author]: https://huggingface.co/moonshotai/Kimi-K3/blob/f831ab66814297da540d832a5235f8e904f29d06/README.md
[models]: https://docs.tokenfactory.nebius.com/api-reference/models/list-models
[tools]: https://docs.tokenfactory.nebius.com/ai-models-inference/function-calling
