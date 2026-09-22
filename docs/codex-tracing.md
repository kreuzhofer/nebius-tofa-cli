# Opt-in Codex request tracing

The released launcher has no `--debug` flag. From this repository, run the
standalone Python harness to observe the real installed Codex talking to the
real launcher's local request adapter:

```sh
python3 scripts/trace_codex.py \
  --launcher "$HOME/.local/share/tofa/bin/tofa" \
  --codex "$HOME/.local/bin/codex" \
  --output /private/tmp/tofa-approval-trace.jsonl \
  --approval-probe
```

Choose a new output filename for every run. In another terminal, watch the
sanitized records live:

```sh
tail -f /private/tmp/tofa-approval-trace.jsonl
```

The file is also the report to read afterward. This harness currently supports
macOS/POSIX with Python 3.9 or newer; it is not part of the released executable
and does not add a Python dependency to the launcher. It uses saved launcher
login credentials and makes real, potentially billable inference calls.

The approval probe asks Codex to request escalation for one harmless
`printf tofa-approval-probe` command, then stop if review fails. It invokes
`exec --approve-for-me`, which selects automatic approval review and the
workspace-write sandbox, in an empty temporary workspace. It does not request dependency installation or
project changes. The command executes only if Codex's actual approval and
sandbox policy permit it. The model may decline the request, so a successful
session exit alone does not prove the automatic reviewer ran. Look for a
request containing `text_format.type: "json_schema"` and tools, followed by the
corresponding response status or error classification.

Use `--prompt 'your diagnostic task'` instead for one other task with the
read-only sandbox in an empty, temporary workspace. Codex receives an isolated temporary HOME and CODEX_HOME;
its normal configuration, sessions, and saved authentication are not loaded.
The launcher retains its normal HOME to read its saved login. Scratch files,
including Codex's own session artifacts, are removed after the run. This does
not attach to an existing Codex conversation or recover a previous wire trace.

The observer receives only the adapter's local bearer token, never the upstream
Nebius key. It accepts authenticated POST requests to `/responses` only and
forwards solely to the exact numeric loopback adapter endpoint supplied by the
launcher. It uses `http.client`, without ambient HTTP proxies or redirect
following. Request bodies and response body bytes pass through unchanged;
response chunks flush immediately. The observer reconstructs HTTP framing and
forwards the authorization, content type, and accept headers required for this
route; it does not preserve all HTTP headers.

The exclusive-created JSONL file has mode `0600`. It records:

- Local sequential request numbers, request stages, HTTP status, and elapsed milliseconds.
- Allowlisted public model identifiers and tool names/types, including namespace
  tools in ResponsesLite `input` items of type `additional_tools`.
- `text.format` type and Boolean strictness, categorical tool choice, and input
  item type/role counts.
- Allowlisted SSE event types/counts, `[DONE]` counts, and a fixed
  `tools_schema_conflict` classification for the known constrained-decoding error.

Unknown names and categories become `other`. No prompts, generated text, tool
arguments/results, tool descriptions, schemas, raw error messages, headers,
keys, tokens, project/session identifiers, or private paths enter the trace.
Client stdout/stderr is discarded; the terminal receives only a sanitized
session summary. A trace can therefore identify the failing request shape but
cannot reconstruct the conversation or expose the full backend error.

Limits are explicit: 24 accepted requests, 16 MiB per request, 32 MiB per
response, 100 tool metadata entries per request, and the first 128 SSE events
as individual live rows per request. Final rows include aggregate event counts.
Individual event lines exceeding 256 KiB may lose event metadata, marked
`events_truncated`; their response bytes still pass through within the response
limit. By default the whole session is bounded to 600 seconds, and each adapter
request has a **total** budget of 300 seconds, shared between waiting for headers
and reading the response stream. `--timeout` and `--request-timeout` select
smaller operator budgets within those caps. These are diagnostic limits, not
expectations about provider response times. Nebius latency can vary; reaching
a diagnostic timeout leaves the diagnosis incomplete and does not establish
an API error or compatibility failure.

Request records flush before forwarding. `stage: "response_headers"` means the
observer is waiting for adapter headers; `stage: "stream_read"` in a timeout
record means it was waiting for more response bytes after headers arrived.
A slow header response consumes part of the same total request budget. For
example, headers arriving after 84 seconds under an operator-selected 90-second
budget leave about six seconds to observe streaming. Timeout or size limits may
terminate the diagnostic request. A terminated process may leave a request
without its final response row. The session's terminal summary reports timeout
and exit status.

Run the offline fixture tests with:

```sh
python3 -m unittest discover -s scripts -p trace_codex_test.py
```

Tests use local loopback HTTP servers and synthetic credentials only.
