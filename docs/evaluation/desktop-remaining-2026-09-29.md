# Remaining desktop model evaluations

[Issue #63](https://github.com/kreuzhofer/nebius-tofa-cli/issues/63) continues
the [DeepSeek/Kimi campaign](desktop-first-pair-2026-09-29.md) for
`zai-org/GLM-5.3-Flash`, `zai-org/GLM-5.3` and
`nvidia/Nemotron-3-Ultra-550b-a55b`, each with `zai-org/GLM-5.3-Flash` Guardian.
The live runs use the retained final source, launcher and bundled engine from
#62. Results qualify the desktop launcher's routing and headless bundled-engine
interaction; Electron UI behavior and broad coding quality are outside this
workload. The launcher's supported-model policy is unchanged.

**GLM passed the complete baseline. GLM Flash failed a file correctness check;
Nemotron failed during tool-result continuation. All three Guardian lanes passed.**
Together with the earlier DeepSeek pass and Kimi deadline failure, every priority
candidate now has an evidence-backed outcome. All five passing was not required.

## Outcomes

| Exact main model | Two-turn coding sessions | Resume/relaunch | Guardian allow/deny pairs | Final-source baseline outcome |
| --- | --- | --- | --- | --- |
| `deepseek-ai/DeepSeek-V4.1-Flash` | 3/3 passed | 3/3 passed | 3/3 passed | Passed, retained #62 result |
| `zai-org/GLM-5.3-Flash` | 2 passed; third failed on turn 1 | 2 passed; third unattempted | 3/3 passed | Failed: file correctness/integrity check |
| `zai-org/GLM-5.3` | 3/3 passed | 3/3 passed | 3/3 passed | Passed |
| `moonshotai/Kimi-K3` | First turn failed; sessions 2–3 unattempted | Unmeasured | 3/3 passed | Failed: main deadline, retained #62 result |
| `nvidia/Nemotron-3-Ultra-550b-a55b` | First turn failed; sessions 2–3 unattempted | Unmeasured | 3/3 passed | Failed: HTTP 422 on tool-result continuation |

Every row uses `zai-org/GLM-5.3-Flash` Guardian. Earlier setup failures and
diagnostics remain in the [complete campaign](evidence/desktop-remaining-2026-09-29/campaign.json).
Its aggregate statuses still show `multiple_conditions` for DeepSeek and Kimi;
the table selects their documented final-source baseline invocations without
erasing other attempts. The three new candidates each have one baseline run.

The [GLM Flash baseline](evidence/desktop-remaining-2026-09-29/glm-flash-baseline.json)
started at **12:06:35 UTC**, run `1ccbafe9ecd84f19812c280506c531ab`.
Its first two sessions passed both turns and preserved conversation, main and
provider identity after relaunch. Session three's first turn completed in
60.795 seconds with successful tool execution, streaming and tool-result
continuation, but `files_correct` was false. That combined check covers expected
summary contents and input preservation; the retained evidence does not distinguish
which failed or retain the incorrect file. Its continuation was unattempted.
All three independent Guardian allow/deny pairs passed. This is a failed common
baseline for the same-model pair despite working protocol and approval checks.

The [GLM baseline](evidence/desktop-remaining-2026-09-29/glm-baseline.json)
started at **12:15:31 UTC**, run `5ce8bf399c064c40b8ff8c675e481469`.
All three two-turn sessions passed, including independently checked files,
streaming, successful tools, tool-result continuation and same-conversation
resume after relaunch. All three Guardian allow/deny pairs passed.

The [Nemotron baseline](evidence/desktop-remaining-2026-09-29/nemotron-baseline.json)
started at **12:18:46 UTC**, run `a8d15ec220894f38a6dbd7abace47ad1`.
Its first request streamed tool-call deltas, completed with HTTP 200 and produced
one successful tool execution. The request carrying that tool result received
HTTP 422, followed by observer failure `event_body_limit` while reading the
response. The turn failed after 2.910 seconds; no visible text, successful result
continuation, or resume was established. The remaining coding work was unattempted.
All three Guardian pairs passed. The sanitized evidence does not retain the
provider's error body, so it establishes the rejection and observer limit, not
the underlying request-validation cause. That request's usage and cost are unknown.

For all three new runs, decisions were `allow, deny, allow, deny, allow, deny` and
actual execution was `true, false, true, false, true, false`. There were no added
live diagnostics or reruns: these outcomes are sufficient to complete the common
baseline without claiming either failed candidate is permanently incompatible.

## Frozen conditions and comparability

The [continuation record](evidence/desktop-remaining-2026-09-29/continuation.json)
pins the prior campaign and [final freeze](evidence/desktop-first-pair-2026-09-29/final/freeze.json)
by hash. All retained source, artifact and workload hashes were checked before
inference. This continuation makes no implementation, capability, route or
deadline changes. Each invocation refreshes authenticated project availability,
exact main/Guardian metadata and current prices before attempting inference.

- Launcher: `tofa qualification-62-v2`, built with Go 1.27.1 from base commit
  `9ae176b5dfdfc6d9d98716167e72b0e0aba2946d` plus the retained source patch.
- Client: ChatGPT 26.917.71314 (10954), retained bundled engine
  `codex-cli 0.155.0-alpha.16.4`, macOS 26.6.2 ARM64.
- Route: adapted desktop connection with evaluation observer and disposable
  headless interaction driver.
- Workload: `summary-and-continuation-v1`; three independent two-turn tool
  sessions with relaunch/resume and three real Guardian allow/deny pairs.
- Limits: 180-second coding request/turn, 120-second approval turn, native
  90-second review deadline, 4,096 output tokens, 1 MiB request, 8 MiB response,
  256 KiB SSE event. No spending cap or request quota.

Live runs are sequential. Failed sequences stop; unattempted continuations stay
visible. The Guardian lane runs independently of the coding lane. Approval-case
main proposals are synthetic, so these checks establish native Guardian parsing
and execution gates rather than natural main-model approval behavior. No harness
retry, model substitution or deadline extension is used.

The earlier DeepSeek/Kimi runs took place at 09:58 and 10:02 UTC on the same day.
This continuation uses identical shared conditions, but different run times and
model-specific metadata prevent treating timing differences as controlled model
effects. Full metadata snapshot fingerprints also include refreshed catalog and
price information. Compare role capabilities and shared conditions explicitly;
do not pool differing metadata fingerprints into one identical-condition group.
The fresh controlled-provider regression ran locally during part of GLM Flash's
live baseline; local test load was not measured. The full Go/Python suites ran
after all live baselines finished. This further limits latency comparisons.

## Timings and cost

Observed ranges and medians are seconds. Counts include attempted failed turns;
missing observations are excluded from numeric summaries and identified here.

| Measurement | GLM Flash | GLM | Nemotron |
| --- | --- | --- | --- |
| Main first visible text, per turn | 12.363–33.816; median 25.902, n=5 | 1.263–2.237; median 1.800, n=6 | Missing; no visible text |
| Main whole turn | 41.283–71.121; median 55.839, n=5 | 5.058–8.157; median 6.719, n=6 | 2.910, one failed turn |
| Main request first delta, text or tool | 12.229–36.002; median 22.256, n=12 | 1.052–2.215; median 1.369, n=21 | 2.115, n=1; missing on rejected request |
| Main request stream completion | 12.573–36.222; median 22.519, n=12 | 1.088–2.700; median 1.714, n=21 | 2.161, n=1; missing on rejected request |
| Native Guardian review | 19.166–55.768; median 29.977, n=6 | 5.263–17.292; median 10.089, n=6 | 15.009–27.946; median 19.939, n=6 |

The [earlier report](desktop-first-pair-2026-09-29.md#measurements-and-cost)
retains the same timing categories for DeepSeek and Kimi. Kimi's first turn
reached 180.025 seconds without response headers, visible text, completion or
usage. Main first visible text differs from the observer's first text-or-tool
delta, as Nemotron's tool-only response illustrates. Request starts are relative
to each observer clock, not a shared absolute per-request clock.

Prices were refreshed on 2026-09-29 from the
[Token Factory public catalog](https://tokenfactory.nebius.com/api/public/models_info).
The earlier rows retain their original same-day snapshots. Token counts on rows
with missing usage are measured subtotals only.

| Baseline / role | Paid attempts / measured | Observed input / output tokens | Input / output USD per million | Estimated USD |
| --- | ---: | ---: | ---: | ---: |
| DeepSeek main (#62) | 21 / 21 | 171,740 / 2,815 | 0.30 / 1.20 | 0.05490000 |
| DeepSeek Guardian (#62) | 6 / 6 | 50,492 / 1,039 | 0.15 / 0.50 | 0.00809330 |
| Kimi main (#62) | 1 / 0 | Unknown / unknown | 3.00 / 15.00 | Unknown |
| Kimi Guardian (#62) | 6 / 6 | 50,499 / 1,078 | 0.15 / 0.50 | 0.00811385 |
| GLM Flash main | 12 / 12 | 136,333 / 2,077 | 0.15 / 0.50 | 0.02148845 |
| GLM Flash Guardian | 6 / 6 | 50,495 / 1,311 | 0.15 / 0.50 | 0.00822975 |
| GLM main | 21 / 21 | 149,278 / 3,069 | 1.40 / 4.40 | 0.22249280 |
| GLM Guardian | 6 / 6 | 50,496 / 982 | 0.15 / 0.50 | 0.00806540 |
| Nemotron main | 2 / 1 | 10,647 / 113 | 1.00 / 3.00 | Unknown; known subtotal 0.01098600 |
| Nemotron Guardian | 6 / 6 | 50,498 / 1,111 | 0.15 / 0.50 | 0.00813020 |

The three new runs add **53 paid attempts** (52 with usage), **36 synthetic
observations** and a known subtotal of **USD 0.27939260**. Across all **12 retained
invocations**, there are **147 unique observations**: 87 paid attempts (85 with
usage), 60 synthetic observations and zero auxiliary attempts. Observed totals
are **720,478 input tokens**, **13,595 output tokens**, and **USD 0.35049975** known
cost. Complete usage/cost remains unknown because the Kimi timeout and Nemotron
rejection supplied no usage. Earlier setup failures retain their unknown cost
fields. Synthetic proposals/continuations are excluded from paid accounting.

## Preservation and measurement limits

The driver uses disposable profiles, engine state and synthetic workspaces.
It reads the existing launcher login through the established Keychains directory
link; the engine uses scratch file authentication. Normal settings and credential
files and scratch credential protections are checked by each report. No active
desktop transition is needed and no private conversation is copied into a run.

Sanitized JSON retains exact role IDs, run-start UTC, per-request relative starts,
headers, first deltas, stream completion, whole-turn and native/observer review
durations, usage and prices. Those timings include transport, adapter and local
overhead; pure provider compute time is unknown. Missing usage or prices remains
unknown rather than zero. Approximate cost estimates are not billing totals.

Generated naming is unmeasured and is not a qualification gate. Only the recognized
Kimi naming route is supported; other-main naming remains explicitly unsupported.
The provisional first-message title is not evidence of generated-title success.
The small workload establishes neither an SLA nor a latency or reliability ranking.

## Validation and recovery

[Validation totals](evidence/desktop-remaining-2026-09-29/validation.json) record
12 unique runs and 147 unique request observations. The regenerated campaign
preserves all nine earlier run objects exactly, including failed setup and
diagnostic attempts. Each new report matches the final #62 source, launcher,
engine, manifest, platform, route and workload. Refreshed main/Guardian capability
fields match the corresponding frozen controlled-provider evidence; current
prices are recorded separately. Passed cases were checked against their tool,
file, streaming, conversation-identity and execution observations. Token totals
and priced subtotals were independently recomputed from per-request usage.

[Checks and coverage](evidence/desktop-remaining-2026-09-29/checks.json) record the
observer/evaluator regressions, the installed-engine controlled baseline test,
static checks and final suite results. The retained successful
[GLM Flash](evidence/desktop-remaining-2026-09-29/glm-flash-controlled.json),
[GLM](evidence/desktop-remaining-2026-09-29/glm-controlled.json) and
[Nemotron](evidence/desktop-remaining-2026-09-29/nemotron-controlled.json)
reports are prerequisites for live execution, not extra paid baseline runs.
No production or evaluator source changes were needed for this ticket.

The full Go race suite and `go vet` passed. Python discovery ran 206 tests:
149 passed, 51 skipped, and six failed during setup because the lifecycle and
terminal scripts require their own command-line arguments. Running those two
scripts with the documented arguments passed all six tests, giving 155 passes
and 51 skips with no unresolved failures. The failed discovery invocation stays
in the checks record. Native Windows, optional standalone Codex and Electron UI
coverage remain unavailable here; standalone ownership/picker fixture skips are
covered by their Go parent tests. Both Standards and Spec reviews found no issues.

The private, git-ignored `.qualification/desktop-remaining-2026-09-29/` retains
the three new run directories and report exports. The combined
`.qualification/desktop-campaign-2026-09-29/` links every old and new run exactly
once; its older `report.json` remains the #62 snapshot, so explicitly regenerate
to recover the complete campaign. Source/artifacts stay in
`.qualification/desktop-2026-09-29-v2/`. From the repository root:

```sh
artifact="$PWD/.qualification/desktop-2026-09-29-v2"
python3 "$artifact/source/scripts/model_evaluation.py" \
  --desktop-report "$PWD/.qualification/desktop-campaign-2026-09-29" \
  --output /absolute/path/to/new-campaign-report.json
```

That command makes no inference requests. Exports do not become new accounting
inputs. Any further live investigation must use `--diagnostic`, a new output
filename and the same retained artifacts to keep the baseline intact; changed
conditions require separate evidence. See the [campaign workflow](desktop-campaign.md).

An experimental launch with the newly passing combination is:

```sh
tofa launch codex-desktop --model zai-org/GLM-5.3 \
  --guardian-model zai-org/GLM-5.3-Flash --allow-unverified
```

Arrange any active-desktop transition before an ordinary launch. Existing
conversations retain their recorded main/provider; relaunch with their original
main to resume them. Follow the [desktop recovery instructions](../codex-desktop.md).
Generated naming remains unsupported for this main.
