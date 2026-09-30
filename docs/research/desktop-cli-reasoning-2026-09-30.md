# Desktop and CLI reasoning, 2026-09-30

The user requires correct detection and separation of thinking tokens in both
clients, including retained history and tool calls. Removing visible tags is not
the requested behavior. This investigation changes no product code and runs no
paid inference.

## Evidence and source identities

The installed desktop engine reports `codex-cli 0.159.2`; its identity and
extracted desktop sources are recorded in
[the compatibility audit](desktop-minimum-version-2026-09-30.md). The matching
upstream `rust-v0.159.2` tag resolves through annotated tag
`8b9fa496bbf2c47aebd62e85a080b9a522a455b5` to commit
`ff6aec96948b70d94983af2641a6b67c94faeff5`. Inspected source copies are at
`/private/tmp/tofa159-reasoning-source`. Matching version labels do not prove
identical private build source; native-engine fixtures remain the runtime check.

Parent-run synthetic GLM-5.3 live probes recorded these observations, without
executing any returned tools:

| Request setting | Provider reasoning deltas | Answer text | Usage reasoning tokens |
| --- | --- | --- | --- |
| Effort omitted | 56 `response.reasoning_text.delta` events, 901 characters | 115 characters, no closing thinking tag | 191 |
| `reasoning.effort: none` | None | 999 characters, including closing thinking tag | 0 |
| Repeated `none` | None | 1008 characters, including 865 characters before one closing tag; no opening tag | 0 |

Artifacts: `/private/tmp/tofa-35-glm-tag-probe/report-default.json`,
`report-none.json`, `report-none-repeat.json`, and `report-default-shapes.json`.
The shape probe reports `content_index:0`, item identity/index/sequence fields,
and reasoning items with `content`, `summary`, and `type:reasoning`; final content
uses `type:reasoning_text`. These observations establish a request-setting
correlation for this model and fixture, not a general parser rule for every
model or text containing a literal tag.

## Existing native reasoning contract

The version-matched SSE parser supports both raw reasoning and summaries:
`response.reasoning_text.delta` becomes `ReasoningContentDelta` when `delta` and
`content_index` exist; `response.reasoning_summary_text.delta` becomes
`ReasoningSummaryDelta` with `summary_index`. It independently maps answer text
to `OutputTextDelta` and reads reasoning-token usage from the completed response.
The default probe supplies the required raw-reasoning fields, so renaming raw
reasoning events into summaries is unnecessary.
[Responses SSE parser](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/codex-api/src/sse/responses.rs#L377)

The native response model represents reasoning separately from messages and
function calls. A reasoning item has optional ID/content, a required summary
vector, and optional encrypted content; raw content accepts `reasoning_text` and
`text`, while summary content uses `summary_text`.
[Response item types](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/protocol/src/models.rs#L1048)

Core associates reasoning deltas with the active item and emits
`ReasoningRawContentDelta`; missing active-item lifecycle is an error. The
app-server mapping emits `item/reasoning/textDelta` for raw content and
`item/reasoning/summaryTextDelta` for summaries.
[Core stream handling](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/core/src/session/turn.rs#L3087),
[app-server mapping](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/app-server-protocol/src/protocol/event_mapping.rs#L378)

The installed desktop's `bootstrap-B7ariqxX.js` consumes those two notifications
into separate `reasoningContent` and `reasoningSummary` queue targets, indexed by
item and content/summary index. It does not require raw reasoning to be delivered
as answer text. The CLI has the same protocol distinction but displays raw
reasoning only when `show_raw_agent_reasoning` is enabled. Replay follows that
same display setting. Correct handling therefore does not imply displaying all
raw reasoning by default.
[CLI live handling](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/chatwidget/protocol.rs#L130),
[CLI replay](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/tui/src/chatwidget/replay.rs#L345)

## How absent metadata becomes explicit `none`

The launcher currently provides no `default_reasoning_level`, an empty
`supported_reasoning_levels`, and `supports_reasoning_summary_parameter:false`.
That advertises no verified effort controls. However, native `ModelInfo` to
`ModelPreset` conversion fills an absent default with `ReasoningEffort::None`.
Thus the model-list default can become an explicit selection of `none`.
[Preset conversion](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/protocol/src/openai_models.rs#L943)

The native request builder always builds a reasoning object. Its effort is the
explicit selected effort, falling back to the descriptor default. Only the
summary field is gated by `supports_reasoning_summary_parameter`; that flag
and empty effort choices do not suppress a supplied effort. Effort resolution
passes `none`, `low`, `high`, etc. through; only special UI aliases receive
additional translation. With no selected effort and no descriptor default, the
effort is absent. With an explicit native `none`, it is sent as `none`.
[Request builder](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/core/src/client.rs#L863),
[effort resolution](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/protocol/src/openai_models/reasoning_effort.rs#L10)

This is a source-supported mechanism, not proof of the exact request emitted by
the user's turn. A public launch/request capture must distinguish absent effort,
explicit `none`, and inherited CLI effort such as `high`. Saved turn context is
insufficient to establish the wire body.

The GLM-5.3 author's card lists thinking budgets `low`, `high`, and `max`, with
`max` as the default for omitted or other values. That describes the model's
serving/template contract; it does not establish Nebius Responses mappings or
justify advertising new effort choices. In particular, HTTP success with `none`
is not proof of a supported no-thinking mode.
[GLM-5.3 author card](https://huggingface.co/zai-org/GLM-5.3/blob/main/README.md#note)

## Boundary decision and required verification

**Recommendation/inference:** Correct the demonstrated outbound default-setting
mismatch while preserving the provider's structured reasoning stream. Do not
parse arbitrary thinking tags out of ordinary assistant text, relabel raw
thinking as a summary, invent reasoning-token counts, or select another model.
The provider default already supplies the native reasoning contract in the
observed GLM-5.3 fixture.

There is an unavoidable provenance tradeoff: a Responses `effort:none` value
cannot identify whether the desktop fabricated it or the user deliberately chose
it. A narrow correction can explicitly define GLM-5.3 as provider-managed
thinking, announce that native None is not a verified no-thinking control, and
omit this sentinel at the outbound boundary. It must not silently claim that
thinking was disabled. If explicit None must mean disabled thinking, the honest
behavior is to reject it, including the indistinguishable fabricated default.
Do not extend normalization to other models or reject existing CLI `high`
settings without evidence. Model-specific defaults/control contracts need their
own qualification; changing them all would exceed this reproduction.

Before claiming a fix, verify both public client paths with synthetic streams:

- Capture the actual outbound request for absent/None/inherited effort, including
  a resumed conversation, and assert any normalization is explicit.
- Deliver complete reasoning item start/deltas/end plus answer and function-call
  items. Assert separate native reasoning and assistant events and retained items.
- Verify tool-call arguments, continuation history, cancellation, and usage survive
  unchanged. Cover raw reasoning and summary events as distinct channels.
- Include an ordinary answer containing literal `<think>`/`</think>` text. Preserve
  it byte-for-byte; a closing tag alone is not a reliable structural delimiter.
- Check CLI default visibility separately from optional raw-reasoning visibility;
  desktop UI confirmation remains necessary for its rendering path.

## Approved correction and runtime verification

The user approved provider-managed thinking with a one-time launch-terminal
notice after explicitly rejecting tag stripping as the goal. The shared adapter
now omits only GLM-5.3's `reasoning.effort:none`; it removes an otherwise empty
reasoning object, preserves other reasoning fields and explicit effort settings,
and leaves every response byte unchanged. This applies to desktop and CLI.
The native None label is not a promise that this model's thinking is disabled.

A real desktop-engine capture first confirmed an unselected effort produces
`reasoning:{}`. Supplying the desktop preset's None produced
`reasoning:{"effort":"none"}` and failed the new public-boundary regression.
After the correction, the same test passed. Both raw reasoning and summaries
arrived through their separate native events; an answer containing a literal
`</think>` remained intact. Closing the engine, resuming the conversation and
reading it through the public API retained distinct raw reasoning and summary
items; another turn completed successfully.

The installed CLI is 0.158.0. Its native integration test confirms reasoning usage,
separate summary/answer output and retained raw reasoning history. The initial
observer expected raw reasoning in `exec --json` displayed items, but that mode
omits it. The corrected test uses its actual usage and persisted-history contract;
this observer failure is retained separately. TUI raw-reasoning visibility remains
a native user setting. CLI tool/result continuation and native approval regression
checks also passed with the correction.

The original bounded live probe, requesting None through the corrected adapter,
then returned 54 raw reasoning deltas, 774 reasoning characters, 119 answer
characters and 168 reported reasoning tokens, with no inline closing tag.
Function-call argument events remained present; no returned tool was executed by
the probe. This is a request-contract correction, not postprocessing of generated
reasoning or answer text. The probe used a scratch build; fresh immutable-candidate
UI and release qualification remain necessary.

Standards review caught an unbounded test-handler channel send on excess native
requests. Capture now fails explicitly instead of blocking. The corrected native
and adapter race checks passed in 6.335 seconds. No credentials were changed or
exported by these investigations.
