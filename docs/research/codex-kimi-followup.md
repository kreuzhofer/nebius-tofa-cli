# Codex 0.154.0 / Kimi-K3 follow-up compatibility

Investigated 2026-09-21. Scope: the reported metadata warning and second-turn HTTP 422 from direct Token Factory Responses requests. No remote inference requests, credentials or proxy implementation were used for this research.

## Finding

The evidence establishes a **history serialization mismatch**, separate from the missing-model-metadata warning. Codex 0.154.0 omits `message.status` and `output_text.annotations` when it serializes assistant history. Token Factory's published schema requires both for that output-message representation. OpenAI's published SDK input types also require those members; the evidence does **not** establish that Nebius violates the published OpenAI schema.

No supported Codex setting that supplies these omitted nested history fields was found. A model catalog controls model metadata and behavior; it does not replace the message serializer. The direct integration therefore cannot be called supported on the strength of the successful first reply.

## Evidence and version pins

- **User observation:** `moonshotai/Kimi-K3` starts with a metadata warning, answers once, then Token Factory `/v1/responses` returns 422 identifying missing assistant-history annotations and status. This report did not repeat that paid service request.
- **Local reproduction, performed by the parent implementation agent:** installed Codex 0.154.0 received a synthetic local SSE response containing `status: "completed"` and `annotations: []`. An `exec resume` request omitted both. Each run sent two requests only to the local fixture, using a dummy key and scratch `HOME`/`CODEX_HOME`. The Kimi run showed the metadata warning. A second run selecting `gpt-5.4` removed that warning but omitted exactly the same fields. No real model ran: the fixture supplied both replies. This experimentally separates the metadata warning from the serializer defect. The repository reproduction is [scripts/repro_codex_history.py](../../scripts/repro_codex_history.py), with default Kimi or `TOFA_REPRO_MODEL=gpt-5.4`; exit 1 with `REPRODUCED` denotes detection of the omission. This tests client serialization, not remote service behavior or model inference.
- **Codex source:** tag `rust-v0.154.0`, resolved through GitHub's tree API to commit [`6b9826e3aa83b1a5947db50f4332cb9c65f1b340`](https://github.com/openai/codex/tree/6b9826e3aa83b1a5947db50f4332cb9c65f1b340).
- **Nebius schema:** public [OpenAPI document](https://api.tokenfactory.nebius.com/openapi.json), linked by the [official documentation index](https://docs.tokenfactory.nebius.com/llms.txt), fetched with `info.version = 20260916-b712a99a4`. SHA-256: `e6ba0e6dc303e48fb8eec5d62fcc29cfc2f552a7725a2ae0903719023961ddb5`. This is a documentation snapshot, not proof of the deployed validation implementation.
- **OpenAI SDK:** public `openai-python` main resolved during inspection to [`69a2c1db6feacf32be6693809e7cab1c3b49cad7`](https://github.com/openai/openai-python/tree/69a2c1db6feacf32be6693809e7cab1c3b49cad7). Current official [Responses documentation](https://developers.openai.com/api/reference/python/resources/responses/methods/create) was consulted before source inspection.

## Input and output contracts

Nebius's `CreateResponseRequest.input` accepts text or a union of item types. The relevant branches are:

| Branch | Relevant constraints |
| --- | --- |
| `EasyInputMessage` | Requires `content` and `role`; permits role `assistant`. Content can be a string or input-text/image/file parts. It does not permit `output_text` parts. |
| `Message` | Requires content and role, with user/system/developer roles. Its status is optional. This branch does not provide an assistant-output escape hatch. |
| `ResponseOutputMessage` | Requires `id`, `content`, `role`, `status`, `type`; role is assistant and type is message. Content uses output-text/refusal parts. |
| `ResponseOutputText` | Requires `annotations`, `text`, `type`; type is output_text. An empty annotations array is valid; an absent required member is different. |

These are exact distinctions in the fetched [Nebius OpenAPI schema](https://api.tokenfactory.nebius.com/openapi.json), not conclusions drawn merely from its response example. The human-readable [create-response reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response) also exposes the input union and an output example with both members.

OpenAI's SDK likewise distinguishes an [easy assistant input message](https://github.com/openai/openai-python/blob/69a2c1db6feacf32be6693809e7cab1c3b49cad7/src/openai/types/responses/easy_input_message_param.py) from an [output message passed as input](https://github.com/openai/openai-python/blob/69a2c1db6feacf32be6693809e7cab1c3b49cad7/src/openai/types/responses/response_output_message_param.py). The latter marks status required, and its [output-text parameter type](https://github.com/openai/openai-python/blob/69a2c1db6feacf32be6693809e7cab1c3b49cad7/src/openai/types/responses/response_output_text_param.py) marks annotations required. Required SDK types do not prove how permissively OpenAI's service handles an actual request; no OpenAI inference call tested that here.

## What the pinned client does

In Codex's [protocol message definitions](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/protocol/src/models.rs#L858), `ContentItem::OutputText` holds only `text`. [`ResponseItem::Message`](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/protocol/src/models.rs#L980) contains optional ID, role, content, optional phase and optional internal metadata, with no status member. Both derive serialization/deserialization. Consequently these types cannot retain the two missing members when reconstructing that history representation. The local reproduction corroborates this source-level result.

The inspected [provider configuration](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/model-provider-info/src/lib.rs#L58) supports endpoint/authentication, headers, query parameters, retry and transport settings. It offers no nested assistant-history transformation setting; `wire_api = "chat"` is explicitly rejected. The [HTTP Responses endpoint](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/codex-api/src/endpoint/responses.rs#L102) encodes the typed request. Changing headers, context window, model slug, reasoning effort or transport has not been established as a repair. Using a new session for every prompt would discard the continued-conversation behavior being tested.

Possible direct resolutions are an upstream client change preserving/producing accepted history or a service change accepting this reduced client shape. Neither is implemented or verified here. This incident supplies a concrete reason to reopen the deferred compatibility-adapter question, but it does not prove an adapter must be selected or that adding two defaults would fully support tool/reasoning histories.

## Model catalog: a distinct issue

The [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference) documents `model_catalog_json` as a startup JSON catalog path. Pinned [`load_catalog_json`](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/core/src/config/mod.rs#L2057) parses `ModelsResponse` and rejects an empty models list. Its root is `{"models": [...]}`, not the OpenAI-style `{"data": [...]}` from a general inference model listing.

The pinned [`ModelInfo` definition](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/protocol/src/openai_models.rs#L400) has mandatory non-optional, non-defaulted fields: `slug`, `display_name`, `supported_reasoning_levels`, `shell_type`, `visibility`, `supported_in_api`, `priority`, `support_verbosity`, `truncation_policy`, and `experimental_supported_tools`. Truncation policy contains `mode` and `limit`; reasoning entries contain effort and description. Other fields have optional/default behavior. Shell choices are `unified_exec` or `disabled`; visibility is `list`, `hide` or `none`. This is client-version-specific metadata, not a portable vendor catalog format.

The pinned [fallback descriptor](https://github.com/openai/codex/blob/6b9826e3aa83b1a5947db50f4332cb9c65f1b340/codex-rs/models-manager/src/model_info.rs#L142) logs an unknown-model warning and installs generic assumptions, including a 272,000-token context window. Those values are not evidence about Kimi or Nebius. A fabricated catalog could hide the warning while introducing inaccurate context, tools or reasoning settings. No Kimi catalog is supplied here.

## Authoritative Kimi facts and remaining gaps

Moonshot's own [Kimi-K3 model card](https://huggingface.co/moonshotai/Kimi-K3/blob/f831ab66814297da540d832a5235f8e904f29d06/README.md), pinned to the repository revision reported during inspection, describes native text/image capability and a 1,048,576-token context window. Its usage section says thinking is always enabled, with top-level `reasoning_effort` values `low`, `high`, `max` and default `max`. It requires complete prior assistant messages, including `reasoning_content` and `tool_calls`, when using its documented Chat Completions history.

Those facts describe the author's model/API usage. They do not establish Token Factory's deployed context limit, accepted Responses `reasoning.effort` values, mapping of preserved thinking history, verbosity/summary support, or Codex-specific shell/apply-patch behavior. This bounded search found no authoritative Nebius Kimi-K3 capability contract resolving those gaps. Author model properties must not be promoted silently into provider-specific Codex settings.

The next compatibility evidence should cover continued text conversation, a tool round trip, and preserved reasoning with the exact client/model/service combination. Until then, classify this as an available model with a demonstrated continuation failure, not a supported model. A metadata warning fix and a conversation compatibility fix must be reviewed separately.

## Proposed engineering inquiry

Provide the upstream client and service maintainers with the pinned versions, the local two-turn reproduction and a redacted failing history item. Ask whether direct compatibility should be achieved by preserving full output-item fields in Codex or by accepting Codex's reduced replay shape in Token Factory. Separately ask Nebius for its Kimi-K3 Responses contract for context limits, reasoning efforts and preserved reasoning/tool history. Record the incident in [Validate a directly connected Ollama-like launcher prototype](https://github.com/kreuzhofer/nebius-tofa-cli/issues/11) and use it to scope the [Investigate an optional local protocol adapter for Token Factory](https://github.com/kreuzhofer/nebius-tofa-cli/issues/9). These are proposed follow-ups; this researcher did not contact maintainers, mutate issues or implement a proxy.
