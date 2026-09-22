# Kimi evaluation baseline — 2026-09-22

**The corrected baseline passed all predeclared measurements.** This records
`moonshotai/Kimi-K3` with Codex CLI **0.155.1**, **macOS ARM64**, through the
launcher's adapted connection and evaluation observer. The launcher was built from
commit `430531a`; the [JSON report](evidence/kimi-codex-0.155.1-macos-arm64.json)
contains the actual executable hashes and hashes of all four harness modules,
retained at evaluator commit `4fb1735`.
Task and rubric versions, operational limits and candidate selection are in the
[workflow](README.md). The run started at **2026-09-22 19:22:37 UTC**.

| Measurement | Observed result |
| --- | --- |
| Coding correctness | 2/2 in each of three independent sessions |
| Protocol compatibility | 6/6 in each session: streaming, successful tools, exact session continuation and completed turns |
| Actual automatic-review allow | 3/3 allow decisions; benign command executed in all three |
| Actual automatic-review deny | 3/3 deny decisions; benign command executed in none |
| Upstream requests | 30 of the declared maximum 48 |
| Usage | 225,312 input tokens; 4,943 output tokens; usage present for all 30 requests |
| Observed response headers | 780–1,612 ms across requests; includes adapter/provider/network time |
| Coding time | 62.67 seconds summed across six turns |
| Automatic-review case time | 3.22–4.65 seconds including client startup and controlled task continuation |
| Preservation | All watched normal settings preserved; all scratch config/auth checks passed |
| Failures | No protocol, coding, approval, timeout or harness failures in corrected baseline |

Applying the documented $3/M input and $15/M output reference prices to reported
usage gives an **informational $0.750081 estimate**, not a billing statement. The
maintainer explicitly removed the Token Factory currency limit before execution;
finite request, body, output-token and time limits remained in force.

Approval task inference was synthetic and deterministic; reviewer inference was
real Kimi through the real launcher adapter. The actual installed Codex parsed each
review assessment and enforced the execution gate. This separates automatic review
quality from the ordinary coding tasks. No normal approval policy was modified,
and `review_model` was not used as a Guardian override.

## Retained setup failure

The [earlier attempt](evidence/kimi-codex-0.155.1-macos-arm64-setup-failure.json)
started at **19:20:40 UTC**, recorded **zero upstream requests**, and ended before
any model measurement. The evaluator passed a redundant `-c web_search=disabled`
argument through the launcher. The launcher correctly rejects caller routing/config
flags and already supplies disabled web search itself. Removing that redundant
argument fixed the evaluator; the installed-client fixture now models this guard.

This is a **harness configuration defect, not Kimi incompatibility**. The original
artifact is retained unchanged: its generic `client_incomplete` classification and
`harness_defect: false` predate the diagnosis and must not be read as a model result.
The corrected baseline has a different start time, evidence file and harness hash;
the failed attempt was not silently retried away.

## Interpretation and checks

The evidence meets this workflow's narrow recommendation threshold for the exact
combination and tasks. It does not establish a broad coding ranking, different
platforms/versions, image or large-context support, or arbitrary approval decisions.
It does not change the launcher's supported-model policy or remove
`--allow-unverified`. Availability of the five selected candidates remains separate
from support; only Kimi was evaluated here.

Offline verification includes nine public scoring-CLI tests, five loopback proxy
boundary/deadline tests, and an installed-Codex fixture that checked all six
synthetic allow/deny execution outcomes. All 19 compatibility regression tests passed, as did Python compilation,
the Go workflow contract tests and `git diff --check`. CI runs the new scoring and
proxy suites without credentials or live inference.

A subsequent review tightened the success gate: a request reporting an SSE error
or incomplete response cannot pass merely because a later `response.completed`
arrives. CLI/HTTP regressions failed before the correction and passed afterward.
The retained baseline predates that stricter predicate; its 30 requests had no
recorded failures, so the observed result is unaffected. Its original artifacts
and module hashes remain unchanged rather than claiming a new live run.

The same review also corrected the 256 KiB SSE-event guard to reject an oversized
complete line before JSON parsing, as well as an oversized partial line. A local
HTTP regression reproduced the bypass and passed after the fix. This changes only
the handling of oversized events; the baseline artifacts remain the original run.
