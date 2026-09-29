# Supported desktop choices and five-model comparison

[Issue #64](https://github.com/kreuzhofer/nebius-tofa-cli/issues/64) promotes
**DeepSeek V4.1 Flash and GLM 5.3**, each with **GLM 5.3 Flash Guardian**, from
retained passing desktop baselines. Source builds expose these two combinations
in the normal picker without `--allow-unverified`. Current project availability
and compatible metadata remain required. This does not promote CLI combinations,
other Guardians, direct connections, other desktop versions or platforms.

## Scope and promotion decision

Support covers the adapted desktop launcher and headless bundled-engine workflow
on **ChatGPT 26.917.71314 (10954), codex-cli 0.155.0-alpha.16.4, macOS 26.6.2
ARM64**. The existing desktop discovery checks enforce those versions and
platform before launch. Electron UI behavior, generated naming, broader coding
quality and release qualification are outside this evidence.

The agreed gate is three independently checked two-turn coding sessions with
streaming, successful tools, tool-result continuation and same-conversation
resume after relaunch, plus three real Guardian allow/deny pairs with matching
execution/nonexecution. Both promoted pairs pass every gate. Guardian proposals
are synthetic to isolate native review parsing and execution enforcement; they
do not prove natural approval behavior from the main model. Naming is not a gate.

The [bundled support records](../../internal/tofa/assets/model-verification.json)
name the exact target, route, roles, baseline report and run ID. The measured
artifact remains `tofa qualification-62-v2`, SHA-256
`cc464955e1ba668ee1154486c1bfd8a10ea5832a45d7461f5dbfb5a450344f0c`.
Its [freeze](evidence/desktop-first-pair-2026-09-29/final/freeze.json) pins the
source base plus patch, launcher, engine, metadata and common workload. The
support-record build is a **later artifact**, not the binary that made the paid
measurements. [Promotion checks and provenance](evidence/desktop-support-2026-09-29/checks.json)
identify that later artifact and regression coverage. No retained campaign report
is rewritten and historical CLI evidence remains experimental.

## Complete candidate matrix

All rows use exact Guardian `zai-org/GLM-5.3-Flash`. Counts below select the five
final-source live baseline invocations; earlier attempts are accounted for below.
“Unattempted turns” includes a stopped session's continuation. “Incomplete” means
an attempted turn failed to complete by its deadline; it is included in failures,
not an additional attempt. No candidate remains blocked or unattempted as a whole.

| Exact main / retained baseline | Coding sessions passed / failed / unattempted (of 3) | Coding turns passed / failed / unattempted (of 6) | Incomplete turns | Resume passed / unattempted (of 3) | Guardian pairs | Outcome and blocker | Generated naming |
| --- | --- | --- | ---: | --- | --- | --- | --- |
| [deepseek-ai/DeepSeek-V4.1-Flash](evidence/desktop-first-pair-2026-09-29/final/deepseek-baseline.json) | 3 / 0 / 0 | 6 / 0 / 0 | 0 | 3 / 0 | 3/3 passed | **Supported**; no remaining baseline blocker | Unsupported |
| [zai-org/GLM-5.3-Flash](evidence/desktop-remaining-2026-09-29/glm-flash-baseline.json) | 2 / 1 / 0 | 4 / 1 / 1 | 0 | 2 / 1 | 3/3 passed | Experimental; third session failed file correctness/input-integrity check | Unsupported |
| [zai-org/GLM-5.3](evidence/desktop-remaining-2026-09-29/glm-baseline.json) | 3 / 0 / 0 | 6 / 0 / 0 | 0 | 3 / 0 | 3/3 passed | **Supported**; no remaining baseline blocker | Unsupported |
| [moonshotai/Kimi-K3](evidence/desktop-first-pair-2026-09-29/final/kimi-baseline.json) | 0 / 1 / 2 | 0 / 1 / 5 | 1 | 0 / 3 | 3/3 passed | Experimental; first main turn reached 180-second deadline without headers or usage | Recognized route only; unmeasured here |
| [nvidia/Nemotron-3-Ultra-550b-a55b](evidence/desktop-remaining-2026-09-29/nemotron-baseline.json) | 0 / 1 / 2 | 0 / 1 / 5 | 0 | 0 / 3 | 3/3 passed | Experimental; HTTP 422 on tool-result continuation, then observer `event_body_limit` | Unsupported |

Across these baselines: 8 passed, 3 failed and 4 unattempted coding sessions;
16 passed, 3 failed and 11 unattempted coding turns; all 15 Guardian pairs passed.
GLM Flash's combined check does not distinguish incorrect output from altered
input. Nemotron's sanitized report retains no provider error body, so the request
validation cause remains unknown. A failed bounded baseline is not proof of
permanent incompatibility. Provisional first-message titles are not generated
naming successes; there is no hidden Kimi naming fallback for other mains.

The [complete campaign](evidence/desktop-remaining-2026-09-29/campaign.json)
retains **12 invocations**: these five, four earlier setup-failed baselines and
three setup diagnostics. The seven earlier invocations each have two failed cases
and seven unattempted cases, with no inference observations. Across all 108 case
records (coding sessions plus individual approval cases), there are 38 passed,
17 failed and 53 unattempted. The underlying failure explanations remain in the
[first-pair report](desktop-first-pair-2026-09-29.md#earlier-failures-and-qualification-checks):
disposable HOME hid the saved Keychain login, then relative campaign paths
prevented child observation persistence. The retained fix and absolute paths
resolved these setup blockers. DeepSeek/Kimi aggregate statuses remain
`multiple_conditions`; DeepSeek's final-source group remains `mixed` because its
setup failure is retained alongside the pass. Promotion cites the reviewed
successful invocation, not a manufactured all-pass campaign aggregate.

## Separated timings

Seconds shown as range; median; observation count. Failed attempted turns are
included where observable. Missing measurements are not zero. The contemporaneous
Kimi baseline uses the same tasks, final source, artifacts and limits as DeepSeek.
The later three candidates continue that freeze on the same day.

| Measurement | DeepSeek | GLM Flash | GLM | Kimi | Nemotron |
| --- | --- | --- | --- | --- | --- |
| Main first visible text per turn | 1.148–8.520; 1.317; n=6 | 12.363–33.816; 25.902; n=5 | 1.263–2.237; 1.800; n=6 | Missing | Missing |
| Main whole turn | 4.467–9.987; 5.839; n=6 | 41.283–71.121; 55.839; n=5 | 5.058–8.157; 6.719; n=6 | 180.025; n=1 failed | 2.910; n=1 failed |
| Main request first text/tool delta | 0.962–2.564; 1.317; n=21 | 12.229–36.002; 22.256; n=12 | 1.052–2.215; 1.369; n=21 | Missing | 2.115; n=1; rejection missing |
| Main request stream completion | 1.132–3.260; 1.721; n=21 | 12.573–36.222; 22.519; n=12 | 1.088–2.700; 1.714; n=21 | Missing | 2.161; n=1; rejection missing |
| Native Guardian review | 1.662–48.194; 15.296; n=6 | 19.166–55.768; 29.977; n=6 | 5.263–17.292; 10.089; n=6 | 1.346–4.080; 2.786; n=6 | 15.009–27.946; 19.939; n=6 |

Run starts (UTC, 2026-09-29): DeepSeek 09:58:05, Kimi 10:02:20, GLM Flash
12:06:35, GLM 12:15:31, Nemotron 12:18:46. Per-request relative starts, response
headers, deltas, completion, engine setup, whole-turn and native/observer review
measurements remain in the JSON. Request first delta can be a tool call before
visible text. Transport, provider wait, adapter and local overhead are included;
pure provider compute is unknown. Refreshed role metadata and prices differ by
candidate and invocation. A local controlled regression overlapped part of GLM
Flash's run without load measurement. These are observations, not a controlled
performance ranking, reliability estimate, or SLA.

## Shared usage and cost

Dated 2026-09-29 price snapshots are retained per run, in USD per million tokens.
These are usage-based estimates, not billing totals. Missing usage stays unknown.

| Baseline role | Paid attempts / with usage | Input / output tokens | Input / output price | Estimated USD |
| --- | ---: | ---: | ---: | ---: |
| DeepSeek main | 21 / 21 | 171,740 / 2,815 | 0.30 / 1.20 | 0.05490000 |
| DeepSeek Guardian | 6 / 6 | 50,492 / 1,039 | 0.15 / 0.50 | 0.00809330 |
| GLM Flash main | 12 / 12 | 136,333 / 2,077 | 0.15 / 0.50 | 0.02148845 |
| GLM Flash Guardian | 6 / 6 | 50,495 / 1,311 | 0.15 / 0.50 | 0.00822975 |
| GLM main | 21 / 21 | 149,278 / 3,069 | 1.40 / 4.40 | 0.22249280 |
| GLM Guardian | 6 / 6 | 50,496 / 982 | 0.15 / 0.50 | 0.00806540 |
| Kimi main | 1 / 0 | Unknown | 3.00 / 15.00 | Unknown |
| Kimi Guardian | 6 / 6 | 50,499 / 1,078 | 0.15 / 0.50 | 0.00811385 |
| Nemotron main | 2 / 1 | 10,647 / 113 measured | 1.00 / 3.00 | Unknown; known subtotal 0.01098600 |
| Nemotron Guardian | 6 / 6 | 50,498 / 1,111 | 0.15 / 0.50 | 0.00813020 |

The shared campaign contains **147 unique observations: 87 paid attempts (85 with
usage), 60 synthetic observations and zero auxiliary attempts**. Observed totals
are **720,478 input tokens, 13,595 output tokens, USD 0.35049975 known subtotal**.
The total cost is unknown due to Kimi timeout and Nemotron rejection; earlier
setup-failure reports retain their unknown cost fields. Exports are not counted
again. The promotion regressions use loopback providers and synthetic credentials:
**zero additional paid requests or provider charges**. Their synthetic usage is
not merged into the live campaign.

There is no campaign spending cap or request quota. The frozen evaluation keeps
180-second coding requests/turns, 120-second approval turns, native 90-second
review deadline, 4,096 output tokens, 1 MiB requests, 8 MiB responses and 256 KiB
SSE events. Cancellation and failure-stop behavior remain intact. These are
qualification limits, not newly imposed production launcher limits. No retries,
model substitution or deadline extension were added to produce a passing result.

## Launch, migration and recovery

Build current source; released rc.2 does not contain these records. Use an existing
`tofa auth login`, quit the ordinary desktop, and launch from your workspace:

```sh
go build -o tofa ./cmd/tofa
# Interactive: current catalog filtered to the two supported pairs.
./tofa launch codex-desktop
# Explicit, also valid in scripts without a terminal:
./tofa launch codex-desktop --model deepseek-ai/DeepSeek-V4.1-Flash
./tofa launch codex-desktop --model zai-org/GLM-5.3 --guardian-model zai-org/GLM-5.3-Flash
# An unqualified Guardian override requires explicit opt-in:
./tofa launch codex-desktop --model deepseek-ai/DeepSeek-V4.1-Flash --guardian-model moonshotai/Kimi-K3 --allow-unverified
# Inspect the full available catalog, including models outside the shortlist:
./tofa launch codex-desktop --allow-unverified
```

Up/Down and Enter select; Escape/Ctrl-C cancel and restore the terminal. A missing
supported main is filtered out, not substituted; an unavailable Guardian is an
error. Models outside the shortlist remain visible under opt-in, disabled if
compatible metadata is missing. Unknown metadata is never invented. No Guardian
picker is added. The effective default is GLM Flash; the launcher prints main,
Guardian, route, support status and the unsupported-naming warning.

Scripts must now specify `--model ID`; a saved preference does not bypass selection
and noninteractive omission fails. Bare `tofa` still launches Codex CLI, whose
normal supported list remains empty. CLI diagnostic `--direct` leaves reviewer
selection to the native client, rejects `--guardian-model`, and remains experimental:

```sh
./tofa launch codex --model deepseek-ai/DeepSeek-V4.1-Flash --direct --allow-unverified
```

For recovery, quit the owned desktop and wait for launcher exit. Reopen ordinary
mode to read shared history, or relaunch through tofa with the conversation's
**original main** and reopen that same conversation. A GLM conversation requires
`--model zai-org/GLM-5.3`; choosing DeepSeek does not switch its recorded main.
Unqualified original mains still require `--allow-unverified`. The effective
Guardian applies to Token Factory conversations for that launch. Native-provider
conversations retain their provider. After abrupt exit, quit surviving desktop
processes manually; do not delete history, credentials, descriptors or lock files.
See [desktop lifecycle and recovery](../codex-desktop.md#switching-launch-modes-and-recovering-a-conversation).

## Regression validation

The full Go race suite and `go vet ./...` passed on macOS ARM64. The desktop
terminal parent test passed all 19 cases, including normal production-record
selection and bundled-engine allow/deny execution for both supported mains.
Python ran 206 tests with 149 passes and 57 expected skips; separately invoked
lifecycle and terminal scripts passed all six tests, for **155 passes, 57 skips,
zero unresolved failures**. The installed engine used synthetic responses;
no paid campaign rerun was needed. Full suites ran concurrently, so their runtime
is not model-latency evidence.

Native Windows/ConPTY, optional standalone Codex and Electron UI coverage remain
unavailable in this run. Standalone desktop fixture skips are covered by their Go
parent tests. The [checks record](evidence/desktop-support-2026-09-29/checks.json)
retains exact skip reasons, commands, hashes, test-first results and both reviews.
Standards review found no issues; the spec review's aggregate-count typo was
corrected and re-reviewed with no remaining findings.

## Handoff

The five-model campaign is complete with documented outcomes. Independent
Lightning naming can now proceed in [#41](https://github.com/kreuzhofer/nebius-tofa-cli/issues/41),
using the unsupported-naming rows and contemporaneous timing limits here; it needs
its own contract and qualification. Generated naming remains unmeasured in this
campaign. [#55](https://github.com/kreuzhofer/nebius-tofa-cli/issues/55) retains
conversation main switching, [#30](https://github.com/kreuzhofer/nebius-tofa-cli/issues/30)
and [#31](https://github.com/kreuzhofer/nebius-tofa-cli/issues/31) retain Claude
integration, and [#35](https://github.com/kreuzhofer/nebius-tofa-cli/issues/35) needs
a new candidate and fresh release qualification. This support publication is not
that release qualification.
