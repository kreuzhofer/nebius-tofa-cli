# Repeatable live compatibility checks

This opt-in developer harness tests the built launcher and installed Codex against
`moonshotai/Kimi-K3` using the launcher's saved project and credentials. It makes
real inference requests. Python 3.9+ is required for this test harness only; the
launcher remains standalone. The harness currently runs on macOS/Linux.

```sh
go build -o ./tofa ./cmd/tofa
python3 scripts/live_compat.py --launcher ./tofa --runs 3 --output /tmp/tofa-live.json
```

The output file must not already exist. Use `--codex /absolute/path/to/codex` to
select a specific installed client. Each turn defaults to a 180-second deadline;
each turn's observer accepts at most 12 requests. These are operational limits.
The harness does not retry failed runs or switch models/routes. Exit zero means
every requested run passed. A nonzero exit means the combination did not qualify;
inspect the sanitized report rather than treating process success alone as proof.

## What a run proves

Each independent run creates a scratch workspace and client configuration. Codex
reads four numbers and creates a JSON summary, executes a check using its shell
tool, then resumes the exact session to extend the summary. Independent literal
expectations check both results. Qualification requires successful tool execution
and completed turns, the same session identity, preserved input/configuration,
and no missing-model-metadata warning.

Codex's `exec --json` output supplies session, tool and turn evidence. It does not
expose incremental text tokens. A test-only executable shim therefore places a
loopback SSE observer between Codex and the launcher's real adapter. It forwards
request/response bodies without changing them, flushes incoming lines promptly,
and records only HTTP status, delta counts and elapsed times. Each turn must show
at least two nonempty text deltas before a response completes. This proves observed
incremental events, not a latency SLA or a benchmark of the uninstrumented route.
The production launcher and its upstream connection are unchanged.

Client closure after a completed response is recorded separately from a transport
failure before completion. The scratch workspace's canonical path is predeclared
trusted so Codex's automatic trust recording does not change the config sentinel.

The observer receives only the random local token supplied to Codex. The real
Nebius key stays with the launcher. Raw client output is consumed without being
retained in the report. Temporary Codex session files and generated task files are
removed when the run ends. The report excludes keys, project IDs, prompts, response
bodies, tool output and session IDs. A process killed abruptly may leave its private
temporary directory behind.

The harness compares in-memory hashes of normal Codex `config.toml`/`auth.json`
and launcher `config.yml`/`credentials.yml` before/after. It does not edit them,
write/delete credential-store entries, or publish their hashes. The scratch Codex
process uses an allowlisted environment and workspace-write sandbox with shell
login startup disabled. File preservation does not independently prove native
credential-store lifecycle behavior or every aspect of an ordinary client launch.

## Offline validation

```sh
python3 scripts/live_compat_test.py
```

These executable-boundary tests use a fake launcher/client and local synthetic
SSE with dummy tokens. They cover successful repeated runs, incorrect files,
failed tools, a wrong resumed session, absent/empty streaming, metadata warnings,
timeout reporting, completed-response disconnects, scratch trust recording, and
report redaction. They never read real credentials or call
Token Factory. Unix CI runs these tests; it never runs the live harness.

The harness follows [official Codex non-interactive execution and explicit session
resume](https://learn.chatgpt.com/docs/non-interactive-mode), checked against the
installed client's help. Evidence applies only to the exact model/client/platform
and route recorded. It does not establish other models, platforms, image support,
context limits, reasoning controls or general coding quality. This prototype
retains its explicit `--allow-unverified` gate; a report does not edit a registry.
