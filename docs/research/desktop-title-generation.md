# Kimi desktop title generation

Implementation and qualification for [#33](https://github.com/kreuzhofer/nebius-tofa-cli/issues/33).

This is historical evidence for the older isolated-profile combination below.
The [#52 shared-profile investigation](desktop-shared-title-generation.md) records
the current client's different model selection and request shape, correction,
and live generated-title qualification. The #33 results remain unchanged.

## Configuration and source contract

The [vendor configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
documents custom providers and `model_catalog_json`. It does not document a
dedicated automatic conversation-title model or disablement setting. Terminal
window title configuration does not control conversation naming.

The inspected installed desktop is ChatGPT 26.915.31945 (9922), bundle
`com.openai.codex`, with Codex engine 0.155.0-alpha.9.2, on macOS 26.6.2 arm64.
The following source findings were rechecked against installed `app.asar` members,
byte-for-byte matching the copies used in the [naming investigation](lightning-chat-naming.md):

| Member | SHA-256 |
| --- | --- |
| `.vite/build/src-C3YaUE83.js` | `14c8c23e8b8dfa874d3fb5a50d54fb28eccf55fb83232c3ab29cb7c0ef0a0472` |
| `.vite/build/main-DUHZj4_w.js` | `9e8a3bd79c817064f28693ca26aa1378895e07ab2108c78c42d0ea20dac9d66e` |

**Source inferred:** `ThreadMetadataGenerationService.generateTitle` calls `ece`,
then `X9` and `z8`. The helper selects the fixed native auxiliary model with static
provider-catalog fallback, marks both source and trigger `thread_title`, and
supplies its own feature overrides and output schema. It does not read a dedicated
title-model, tool-suppression or title-disablement option. `bN` collects the final
message; `xN` parses JSON and validates it against `G9`; `ece` normalizes the result
with `Bne` and `Y9`. Invalid JSON, missing fields, incorrect types, an empty title,
or a title longer than 36 characters do not produce a valid title. This is the
installed implementation, not a vendor promise about future clients.

The pinned [Ollama launcher](https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/cmd/launch/codex_app.go)
and [router](https://github.com/ollama/ollama/blob/6383a0fa9cbf97494b847226e189f6e36b401a08/internal/proxy/codex_desktop.go)
provide catalog/native routing and separate automatic-review handling. They do not
establish a title-specific setting for this custom Token Factory provider. Changing
the catalog default to a different auxiliary model would also affect other helper
requests, as #41 documented; this implementation preserves model selection.

## Reproduction and narrow adaptation

**Empirically captured:** a fresh human-operated desktop conversation generated
the [complete sanitized title request](../../internal/tofa/testdata/desktop-title-request.json).
It selected Kimi, used streaming Responses, `tool_choice: auto`, reasoning effort
`low`, and `codex_output_schema` with `strict: true`. Both `title` and `description`
are required strings; title length is 1–36, description length at least one, and
additional properties are forbidden. There are 11 tool entries / 13 callable
tools, including `mcp__node_repl`. The existing adapter returned its tools-plus-schema
error when the captured request was replayed through the launcher regression.

The adapter now recognizes only Kimi requests with both exact title source/trigger
markers, the captured schema/format and automatic tool choice, and the captured
tool names/types/namespace membership. It moves the complete schema into appended
final-answer instructions. All tool definitions, original instructions, input,
reasoning, selected model, metadata, streaming and other fields remain intact.
The adaptation is announced once per launch. Unsupported shapes fail explicitly;
auxiliary model IDs still do not establish request identity or bypass routing gates.

This deliberately replaces upstream constrained decoding with schema guidance
plus the desktop's existing final-answer validation. It does not guarantee that
every model answer will be schema-valid. Tools are retained because the actual
title prompt can require a read-only app lookup for opaque resources. Adding app
tools or otherwise changing the recognized inventory requires separate evidence
and qualification; it currently fails closed. No response is repaired, invented,
or retried by this adaptation. Malformed output and upstream errors reach the
client unchanged. Guardian adaptation and its execution gates remain separate.

## Validation

HTTP-boundary regressions exercise the captured request, source/schema/tool
variants, unchanged other-model behavior, streaming, malformed title responses,
failed events and upstream 400/429/503 responses. The installed Codex 0.155.1
Guardian regression also passed against synthetic upstream responses: allow,
allow after an inspection tool, deny, malformed/missing/invalid decisions,
upstream failure and cancellation preserve the actual command-execution gate.
That is an offline engine qualification, not a paid Guardian decision.

Live desktop qualification uses the #29 diagnostic route with the source-built
launcher, separate Codex/Electron state and saved launcher credentials. The
diagnostic limits are six cumulative requests, 128 KiB per request and 1,024
output tokens per request, with a ten-minute owned-instance deadline. The real
provider key remains inside the launcher; the bridge only sees its temporary
loopback credential.

**Empirically verified:** the new thread obtained the generated name **Explain
Python calculator addition** (34 characters), persisted by the desktop. The model
also returned the required description. Four requests completed with HTTP 200 and
`response.completed`: title, main tool call, tool-result continuation and a second
user turn. The real `exec_command` ran `printf 'tofa-title-33-ok'`; the user reported
that marker after asking the follow-up. Totals were 39,965 input tokens (including
20,864 cached) and 406 output tokens; title generation used 8,140 input / 46 output.
The live bridge recorded output deltas and upstream request IDs. The generated
name was verified in the isolated desktop state; the user did not separately
confirm the displayed sidebar text. [Sanitized evidence](evidence/codex-desktop-title-2026-09-23.json).

Before cleanup, the user started an additional conversation, reaching the six-request
limit. Those last two streams received HTTP 200 headers but were interrupted during
cleanup, so their completion and usage are unknown. Their content is excluded from
the synthetic qualification; categorical request accounting remains in the evidence.
The token totals above cover only the four completed qualification requests.

The diagnostic wrapper reported a process-group permission error during cleanup
after SIGTERM. An independent process listing found no remaining owned group
processes, the diagnostic listener was closed, and the launcher exited. Scratch
conversation state was retained. Ordinary configuration and credentials were not
written; this run did not perform a before/after digest audit or independent
ordinary-account functionality check.

This verifies one synthetic title in the pinned combination. Different tool
inventories, client versions and app-backed title lookups remain unqualified.
There was no live Guardian decision in this desktop run; the installed-client
offline regression supplies the execution-gate evidence.

Final checks passed: `go test -race ./...` with `TOFA_TEST_CODEX` and
`TOFA_TEST_DESKTOP_ENGINE` set, `go vet ./...`, Linux/Windows amd64 builds, the two
native terminal smoke tests and `git diff --check`. The first race-suite attempt
failed only during the existing installed-client test's temporary-directory cleanup
while a plugin clone was writing; that test passed in isolation and the full suite
passed on rerun. Independent standards and spec reviews each reported zero findings.
