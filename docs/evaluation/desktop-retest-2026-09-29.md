# GLM Flash and Kimi diagnostic reruns

Requested after both models disappeared from the supported-only picker. The
launcher now displays supported and experimental choices together and requires
explicit confirmation before launching an experimental pair. This UI correction
does not promote any model's support status or invalidate earlier passing runs.

The reruns use the same three-session, two-turn workload, pinned desktop engine,
GLM 5.3 Flash Guardian, 180-second main-turn deadline, and failure-stop policy as
the [five-model comparison](desktop-comparison-2026-09-29.md). Each main first
passed a matching full controlled-provider qualification through the real engine.
These are headless desktop-engine tests, not standalone CLI or Electron UI tests.
Provider metadata and prices were refreshed before each live invocation.

## GLM 5.3 Flash repeat

The [repeat](evidence/desktop-retest-2026-09-29/glm-flash-repeat.json) failed its
first coding turn. It completed in **55.128 seconds**, with one successful shell
tool, no failed tools, and two complete HTTP 200 model responses. Tool-result
continuation and streaming both worked. The input file was unchanged, but the
checker could not read `summary.json`. This is a failure to deliver the required
workspace artifact, not a request timeout, tool-execution error, or launcher crash.

The first diagnostic version records all file-read errors as `unreadable`; that
record alone does not establish whether the file was absent or inaccessible.
The affected coding lane stopped: two later sessions and all second conversation turns were
unattempted. Independently, **all three Guardian allow/deny pairs passed** with
the expected execution and nonexecution. Main and Guardian are different roles;
Flash's main failure does not negate its passing Guardian evidence.

The earlier run passed two full sessions, then failed its third first turn after
one successful tool. Its original combined file check cannot distinguish missing
or wrong output from changed input, and its scratch files were not retained.
The new observation narrows the failure class without retroactively claiming the
exact contents or cause of that historical failure.

## Kimi repeat

The [Kimi repeat](evidence/desktop-retest-2026-09-29/kimi-repeat.json) reached its
**180.143-second first-turn deadline** after one successful
tool, with no tool-execution error or client startup error. Unlike the earlier
run, it did receive a complete first response: headers at **120.463 seconds**,
first delta at **121.092 seconds**, completion at **123.259 seconds**. The next
request carried the tool result but received no headers before turn cancellation.
The input remained unchanged; the required summary was unreadable at cancellation.

This isolates the observed blocker to the inference/continuation part of the
turn. It is not the three-second desktop ownership handshake. The observer sits
between engine and adapter, so it cannot separate provider queue/compute time
from network and adapter time, or establish the provider's internal reason for
the delay. All three independent GLM Flash Guardian allow/deny pairs passed,
with the expected execution/nonexecution. Two later coding sessions and all
second conversation turns remained unattempted after the first-turn failure. The adapter does not retry or substitute another model. Extending the
deadline might change the result but was not done in this bounded repeat.

Earlier passing [CLI](kimi-baseline-2026-09-22.md) and
[desktop](../releases/desktop-shared-history-final-2026-09-24.md) runs remain valid
for their recorded conditions. The new timeout is not proof that Kimi can never
work. Those historical successes also do not convert this failed baseline into
a supported current desktop pair.

## Focused Flash file diagnostic

The separate [first-session probe](evidence/desktop-retest-2026-09-29/glm-flash-file-diagnostic.json)
did **not** reproduce the unreadable file. Instead, the first turn created the
correct numeric artifact (`count=4`, `total=18`, `max=9`), preserved the input,
and completed two shell tools successfully, but reached **180.071 seconds**
without finishing the turn. Its first three requests completed after **47.375,
47.409 and 49.242 seconds** respectively. A fourth request carrying tool results
was still waiting for response headers when the turn was cancelled.

This demonstrates a second Flash failure mode under the same 180-second limit:
correct file output can still be followed by an unfinished conversation. It does
not prove why the earlier summary was unreadable. The refined checker now
separates absent files, permission failures, malformed JSON and wrong numeric
content, but neither historical unreadable result is relabelled retrospectively.

The [diagnostic freeze](evidence/desktop-retest-2026-09-29/file-diagnostic-freeze.json)
and [driver](evidence/desktop-retest-2026-09-29/first-session-diagnostic.py) identify
this capsule at `.qualification/desktop-file-diagnostic-2026-09-29/`. The driver
records its hash and `first-coding-session-only` workload, invokes the normal
bounded evaluator, and deliberately leaves the two later coding sessions and six
Guardian cases unattempted. A matching full synthetic qualification passed before
this live probe. This diagnostic is separate from both full reruns and cannot
establish a passing full baseline.

## Provenance and limits

The [freeze](evidence/desktop-retest-2026-09-29/freeze.json) identifies the launcher,
engine and source patch based on `9214539`. The immutable capsule is retained at
`.qualification/desktop-retest-2026-09-29/`, including source and per-request
observations. Its launcher includes the ownership-wait and picker fixes, and the
evaluator adds sanitized file diagnostics. Thus this is a fresh observation of
the same workload, not an identical-source reliability or performance estimate.
The later integer-boundary and file-error classification corrections are not
retroactively substituted into that capsule.

The observer retains statuses, timings, counts, numeric workload fields, and
usage; it does not retain raw prompts, model text, tool output or credentials.
Some local synthetic checks overlapped provider wait time. Timings include
provider wait, transport, adapter and local overhead; provider compute is unknown.
No automatic retry, model substitution, deadline increase or support promotion
was used to obtain a passing result. Normal launcher and client setting hashes
are checked by each run. Paid requests and unknown usage remain in the evidence.

## Accounting and validation

Across the two full reruns, both first coding turns failed and four later sessions
were unattempted. All six Guardian allow/deny pairs passed. There were **16 paid
requests, 15 with usage**, and 24 synthetic observations. Observed usage totals
**130,202 input / 2,767 output tokens**. Known estimated cost is **USD 0.04223820**;
total cost remains unknown because the cancelled Kimi continuation has no usage.
The dated rates are retained per exact role in each report. Estimates are not
provider billing totals. The separate first-session diagnostic is accounted for
separately below and is never counted as a passing full baseline.

Both full reruns passed matching synthetic qualification (three coding sessions
and six Guardian cases each). The refined file-error diagnostic also passed its
own matching full synthetic qualification. File-diagnostic regression tests pass,
including changed input versus wrong output, malformed/missing output, rejection
of nonnumeric fields, redaction of arbitrary text/keys, and a 401-digit integer
that previously raised instead of recording a mismatch.

The launcher follow-up passed 27 CLI and 20 desktop PTY tests, targeted race
checks and `go vet`; Linux/Windows cross-builds passed, without claiming native
runtime coverage. The earlier full Go race suite covered the ownership fix and
app-first menu before the final mixed-list confirmation change. Standards and
spec reviews' integer-boundary and stale-help findings were corrected. Normal
settings and credentials were preserved in both full live runs.

The focused probe added **four paid requests, three with usage**, **33,488 input /
642 output tokens** and **USD 0.00534420 known estimated cost**. No Guardian or
synthetic inference was attempted in that probe. Its fourth request was cancelled
without usage. Across all three live invocations: **20 paid requests, 18 with
usage; 24 synthetic observations; 163,690 observed input / 3,409 output tokens;
USD 0.04758240 known subtotal**. Total cost remains unknown for the two cancelled
requests. All three live invocations preserved normal settings and credentials.

No main passed a new full baseline, so support records remain unchanged. Kimi and
Flash are nevertheless available in the interactive picker as explicit
experimental choices; neither was removed from the provider metadata.
