# Token Factory direct protocol compatibility

Research date: 2026-09-21. Research ticket: [Establish direct Token Factory protocol compatibility](https://github.com/kreuzhofer/nebius-tofa-cli/issues/4).

## Scope and evidence standard

This report covers direct client-to-Token-Factory compatibility. Proxy/adapter design, reuse, implementation, and licensing were deferred by the user during research and are outside this report. Browser authorization is a separate research decision.

**Documented** means an inspected first-party document explicitly describes a feature. **Inferred** means a proposed integration follows from those facts but has not been tested. **Unverified** means there is insufficient evidence of actual end-to-end behavior. API schemas establish a documented contract, not a tested implementation for every model. No credentials were inspected, authenticated requests made, or paid inference performed.

## Findings that change the plan

Token Factory explicitly documents an OpenAI Responses endpoint. A design premised on “Nebius only supports Chat Completions, therefore Codex always needs translation” is incorrect. Direct Responses is a candidate route that needs a real-client compatibility test. [Nebius Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response)

Anthropic Messages is a different case: Nebius's own Claude Code cookbook describes a translation relay as necessary, and the inspected API index contains no native Anthropic Messages operation. This supports recording a direct-route gap, not a claim that an undocumented endpoint can never exist. [Nebius Claude Code cookbook](https://dev.nebius.com/cookbook/claude-code-token-factory-relay), [API index](https://docs.tokenfactory.nebius.com/llms.txt)

## Endpoint and routing matrix

| Client wire protocol or operation | Direct endpoint on `https://api.tokenfactory.nebius.com` | Evidence and integration status |
| --- | --- | --- |
| Chat Completions | `POST /v1/chat/completions` | Documented; direct candidate for clients supporting custom OpenAI-compatible providers. |
| Responses | `POST /v1/responses` | Documented; direct candidate for Responses clients; feature completeness unverified. |
| Model discovery | `GET /v1/models` | Documented; supports `verbose=true` and optional `ai_project_id`. |
| Anthropic Messages | No native endpoint established | Official cookbook uses translation; direct compatibility unverified and currently not a supported planning assumption. |
| Responses retrieval, cancellation, compaction, WebSocket transport | No routes established by the inspected Nebius index | Keep explicitly unverified; do not invent URLs or treat missing documentation as a negative probe. |

Sources: [Chat reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-chat-completion), [Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response), [model reference](https://docs.tokenfactory.nebius.com/api-reference/models/list-models), [index](https://docs.tokenfactory.nebius.com/llms.txt), [Claude Code cookbook](https://dev.nebius.com/cookbook/claude-code-token-factory-relay).

The documented SDK base URL is `https://api.tokenfactory.nebius.com/v1/`. Raw inference requests use `Authorization: Bearer <credential>` and JSON bodies. The introductory examples use an API key read from `NEBIUS_API_KEY`; that environment variable is an example convention, not an HTTP authentication scheme. Distinguishing API keys from user access tokens, and how a browser session can authorize the launcher, belongs to the auth ticket. [API introduction](https://docs.tokenfactory.nebius.com/api-reference/introduction)

Project scoping is documented as an optional **query parameter**, `ai_project_id`, on all three principal routes. The models reference also documents a 400 response for a missing/invalid project. **Inference:** project-bound API keys and general access tokens may require different explicit-scoping behavior; the exact credential rules are not established here. A launcher must verify that a client's URL builder preserves the query on every request; simply appending it to a provider base URL is not evidence that this works. [Chat reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-chat-completion), [Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response), [model reference](https://docs.tokenfactory.nebius.com/api-reference/models/list-models)

## Direct capabilities and limits

### Chat Completions

The reference documents `messages`, `tools`, `tool_choice`, `reasoning_effort`, `max_tokens`, and `max_completion_tokens`. The latter includes reasoning tokens; `max_tokens` defaults to 8192 when omitted/null. It documents data-only SSE terminated by `[DONE]`, with `stream_options.include_usage` adding usage to the last chunk. Accepted fields do not demonstrate that every model implements every setting. [Chat reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-chat-completion)

Nebius's tool guide describes JSON-schema function definitions, automatic or explicitly selected functions, and feeding execution results back into subsequent messages. Tool execution remains the client's responsibility. The guide demonstrates handling `tool_calls` and correlating tool results with their IDs. Parallel-call behavior still needs per-model verification; multiple possible calls in a schema or example are not a completeness guarantee. [Function calling guide](https://docs.tokenfactory.nebius.com/ai-models-inference/function-calling)

The text-generation examples show `finish_reason`, `tool_calls`, and prompt/completion/total usage. Some examples contain language-level syntax mistakes and old model IDs, so they are useful evidence of payload shape, not copy-and-run release tests. [Text examples](https://docs.tokenfactory.nebius.com/api-reference/examples/text-generation)

### Responses

The reference declares SSE, function tools, parallel calls, reasoning, `previous_response_id`, `store`, and background execution. Its schema also includes custom tools and compaction items. Examples show input/output token usage with cached/reasoning details. Several descriptions retain OpenAI-specific wording. **Inference:** the broad schema is insufficient evidence that Nebius implements all corresponding services or model behavior. [Responses reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-a-response)

The compatibility target is richer than a text stream: OpenAI specifies typed lifecycle, content, and terminal events. A client's successful plain-text completion therefore does not prove compatibility with tool turns or interrupted responses. [OpenAI streaming guide](https://developers.openai.com/api/docs/guides/streaming-responses)

OpenAI function calls carry distinct call identifiers; results reference the call rather than just the tool name. Streaming arguments arrive in fragments. Tests should check multiple calls, repeated calls to one tool, and fragmented arguments rather than assuming one tool per assistant turn. [OpenAI function calling](https://developers.openai.com/api/docs/guides/function-calling)

OpenAI distinguishes manually supplied conversation history from persisted conversation/response state. Its documentation describes `previous_response_id` chaining and request-scoped instructions. Nebius's actual retention, retrieval, and continuation behavior must be verified separately. [OpenAI conversation state](https://developers.openai.com/api/docs/guides/conversation-state)

Reasoning introduces further state: OpenAI describes reasoning items and encrypted content for stateless continuation. Those are not interchangeable with printable reasoning text. **Unverified:** Nebius model-specific reasoning fields, encrypted-item round trips, exact token accounting, and whether clients may safely omit optional reasoning settings. [OpenAI reasoning guide](https://developers.openai.com/api/docs/guides/reasoning)

### Images, structured output, models, and errors

Nebius documents vision inputs through image URLs or base64 image data in Chat Completions. The example model is historical; discover a presently available vision model before testing. This does not establish audio, video, arbitrary PDFs, or desktop computer-use compatibility. [Vision guide](https://docs.tokenfactory.nebius.com/api-reference/examples/vision-capabilities)

The dedicated JSON guide documents both JSON objects and schema-constrained outputs, with model-card capability tags. The Chat reference's response-format description instead mentions only text/JSON objects. Treat this as a documentation inconsistency requiring a focused test; do not globally disable structured output based on the narrower description. [JSON guide](https://docs.tokenfactory.nebius.com/ai-models-inference/json), [Chat reference](https://docs.tokenfactory.nebius.com/api-reference/inference/create-chat-completion)

`GET /v1/models?verbose=true` provides a richer catalog. Examples include exact IDs, `context_length`, modality/tokenizer, quantization, pricing, and limits. These are better discovery inputs than copied model names. The shown rich catalog is not an exhaustive protocol-capability matrix. **Recommendation:** retain discovered IDs exactly, scope/cache by account/project, and maintain tested feature assertions separately from raw catalog metadata. [Model examples](https://docs.tokenfactory.nebius.com/api-reference/examples/list-of-models)

The model endpoint documents 400, 401, 403, 422, and 500 responses, including a `detail` error envelope. An OpenAI-compatible happy-path schema therefore does not justify assuming that every error has OpenAI's usual envelope. [Model reference](https://docs.tokenfactory.nebius.com/api-reference/models/list-models)

Rate limits are dynamic; Nebius documents 429 responses and remaining-token/request headers, plus successful over-limit handling when spare capacity exists. Retry timing, partial-stream failure, and interruption billing remain unverified. A launcher should not replay an agent turn after visible output without understanding the client's retry behavior. [Rate-limit guide](https://docs.tokenfactory.nebius.com/ai-models-inference/rate-limits)

### Anthropic-only clients: preserved deferred gap

The already-inspected cookbook reports a working Claude Code route through a relay and calls out streaming tool-argument conversion as essential. This report preserves only the finding that endpoint replacement alone does not establish compatibility. No adapter source/license comparison or design was performed after the scope change. [Nebius cookbook](https://dev.nebius.com/cookbook/claude-code-token-factory-relay)

Anthropic's own streaming contract has message/content-block events and JSON argument deltas, unlike Chat Completions. This supplies a concrete protocol reason to keep the direct-route gap explicit. [Anthropic streaming specification](https://platform.claude.com/docs/en/build-with-claude/streaming)

## Verification plan for the later implementation decision

The following is a proposed plan, not a claim that these tests have passed.

1. **Offline request capture per actual client/version:** use a loopback fixture server with dummy credentials to observe method, joined path, query preservation, auth header, model, reasoning options, tool schemas, and transport. Confirm `ai_project_id` survives model-list and inference operations independently. Fail tests if the client falls back to its normal provider.
2. **Offline streaming fixtures:** fragment SSE frames at arbitrary byte boundaries; exercise text, reasoning where applicable, one/multiple function calls, interleaved argument fragments, Unicode, empty deltas, final usage, incomplete output, server error, abrupt EOF, and cancellation. Validate the real client's parser and its tool-result follow-up request.
3. **State fixtures:** test clients that resend history versus those using `previous_response_id`; cover restart/resume, expiry, project changes, and unknown IDs. Probe whether optional compaction, WebSocket, and hosted-search features can be disabled without losing basic coding work. Do not simulate success for an unsupported mandatory feature.
4. **Future live checks, with explicitly authorized credentials and a bounded spend:** discover the project's catalog; select exact candidate IDs; run text then streaming then a harmless tool round trip. Add parallel tools, structured output, image input, reasoning, resume/compaction, and cancellation only when the target product needs them. Record model, client version, date, request shape, terminal event, usage, and sanitized request IDs.
5. **Compatibility classification:** distinguish documented route, fixture-tested client, live-tested model/client pair, and unsupported mandatory feature. A release should identify the tested combinations instead of advertising universal OpenAI compatibility.

## Questions for Nebius engineering or a controlled live investigation

- Which current models support Responses, and what is the supported subset versus the broad published schema?
- Are response retrieval, cancellation, compaction, background execution, WebSockets, and reasoning-state continuation available? What are their exact routes, retention rules, and constraints?
- Is there a supported native Anthropic Messages route, including streaming/tools, that supersedes the cookbook's relay requirement?
- Which rich-catalog fields provide reliable tool, parallel-tool, vision, JSON-schema, reasoning, and maximum-output capabilities?
- What are the exact error/terminal-stream guarantees, retry hints, request identifiers, and server-side cancellation/billing behavior?
- What are the credential-specific project-scoping rules? The separate auth investigation should settle issuance, browser consent, renewal, revocation, and approved client injection mechanisms.

Recommended planning consequence: keep direct Responses and direct Chat Completions in scope as evidence-backed candidates; make success contingent on client/model compatibility gates. Preserve Anthropic-only gaps for the explicitly deferred effort. This research does not select an implementation language, runtime, model default, or proxy architecture.
