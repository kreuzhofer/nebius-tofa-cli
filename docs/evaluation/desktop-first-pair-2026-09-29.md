# DeepSeek desktop qualification and contemporaneous Kimi baseline

[Issue #62](https://github.com/kreuzhofer/nebius-tofa-cli/issues/62), part of
[#54](https://github.com/kreuzhofer/nebius-tofa-cli/issues/54). Live runs completed
on 2026-09-29, DeepSeek first and Kimi second, with `zai-org/GLM-5.3-Flash` Guardian.

**DeepSeek passed the complete common baseline. Kimi's first main turn reached
the 180-second deadline without a response; its independent Guardian lane passed.**
These are launcher/bundled-engine results using a headless interaction driver,
not Electron UI qualification or a broad coding-quality assessment. The
launcher's supported-model policy is unchanged.

## Outcomes and evidence scope

| Main model | Coding sessions | Resume/relaunch | Guardian allow/deny pairs | Final baseline outcome |
| --- | --- | --- | --- | --- |
| `deepseek-ai/DeepSeek-V4.1-Flash` | 3/3 two-turn sessions passed | 3/3 preserved conversation, main and provider identity | 3/3 pairs passed | Passed |
| `moonshotai/Kimi-K3` | First turn failed; continuation and sessions 2–3 unattempted | Unmeasured | 3/3 pairs passed | Failed: main deadline |
| `zai-org/GLM-5.3-Flash` | Unattempted live | Unmeasured | Unattempted as a main/Guardian combination | Remaining candidate |
| `zai-org/GLM-5.3` | Unattempted live | Unmeasured | Unattempted as a main/Guardian combination | Remaining candidate |
| `nvidia/Nemotron-3-Ultra-550b-a55b` | Unattempted live | Unmeasured | Unattempted as a main/Guardian combination | Remaining candidate |

The [DeepSeek report](evidence/desktop-first-pair-2026-09-29/final/deepseek-baseline.json)
started at **09:58:05 UTC** (run `23f388312ea744cb8a793e1f19ed9c07`). Every turn
streamed, executed successful tools, continued with tool results, produced the
independently expected summary, and preserved its input. Each second turn resumed
the same conversation after a fresh launcher/engine launch.

The [Kimi report](evidence/desktop-first-pair-2026-09-29/final/kimi-baseline.json)
started at **10:02:20 UTC** (run `91005921ae2b42cb926f7cbf4abb415f`). Its first
request reached `deadline_incomplete` while waiting for response headers. No HTTP
status, text/tool delta, stream completion or usage was observed. The turn ended
at 180.025 seconds without tool execution. The affected coding lane stopped;
there was no model substitution or retry added by the evaluator. This establishes
failure under these limits, not permanent Kimi incompatibility or the location of
the upstream delay.

For **each** baseline, approval outcomes were `allow, deny, allow, deny, allow,
deny`; actual marker execution was `true, false, true, false, true, false`.
The native engine enforced its policy and parsed real GLM Flash assessments.
Approval-case main proposals are synthetic: these cases isolate Guardian
execution gates and do not establish natural main-model approval behavior.

## Frozen conditions

The [final freeze](evidence/desktop-first-pair-2026-09-29/final/freeze.json) records
all source file hashes, the exact [source patch](evidence/desktop-first-pair-2026-09-29/final/source.patch)
on base commit `9ae176b5dfdfc6d9d98716167e72b0e0aba2946d`, build command, binary and
engine hashes, source archive hash, platform, route and complete workload manifest.
Both measured baselines used those same artifacts and limits. Each candidate's
availability, exact role metadata and prices were refreshed before inference;
model-specific metadata differs by main, while the Guardian contract is shared.

- Launcher: `tofa qualification-62-v2`, built with Go 1.27.1.
- Installed client: ChatGPT 26.917.71314 (10954); retained bundled engine
  `codex-cli 0.155.0-alpha.16.4`.
- Platform: macOS 26.6.2, ARM64; adapted desktop connection with an evaluation
  observer and disposable headless driver.
- Workload: `summary-and-continuation-v1`; three two-turn sessions with relaunch,
  plus three allow/deny pairs, as in [the common manifest](desktop-baseline.json).
- Limits: 180-second coding request/turn, 120-second approval turn, native
  90-second review deadline, 4,096 output tokens, 1 MiB request, 8 MiB response,
  and 256 KiB SSE event. No spending cap or request quota.

No active desktop transition was necessary. The driver used disposable profiles,
engine state and synthetic workspaces. Normal configuration and credential-file
hashes were unchanged in every recorded run; no native login state or private
conversation was copied into the scratch engine. The launcher read its existing
saved Keychain login through a directory link; the engine used scratch file auth.

## Measurements and cost

Observed timing ranges and medians are seconds, not latency guarantees:

| Measurement | DeepSeek baseline | Kimi baseline |
| --- | --- | --- |
| Main first visible output, per turn | 1.148–8.520; median 1.317, n=6 | Missing; no visible output |
| Main whole-turn duration | 4.467–9.987; median 5.839, n=6 | 180.025, one failed turn |
| Main first request delta, text or tool | 0.962–2.564; median 1.317, n=21 | Missing |
| Main request stream completion | 1.132–3.260; median 1.721, n=21 | Missing |
| Native Guardian review duration | 1.662–48.194; median 15.296, n=6 | 1.346–4.080; median 2.786, n=6 |

The JSON retains run-start UTC, request starts relative to each observer clock,
header/first-delta/completion measurements, whole-turn time, and native review
and observer assessment durations separately. Setup, transport, adapter and
provider wait are not pure provider compute measurements. The variation between
successive Guardian reviews does not establish a model or route latency effect.

Prices below were refreshed on 2026-09-29 from the
[Token Factory public catalog](https://tokenfactory.nebius.com/api/public/models_info).
Costs use observed provider usage at those approximate rates, not billing totals.

| Baseline / role | Paid attempts | Observed input / output tokens | Input / output USD per million | Estimated USD |
| --- | ---: | ---: | ---: | ---: |
| DeepSeek main | 21 | 171,740 / 2,815 | 0.30 / 1.20 | 0.05490000 |
| DeepSeek Guardian | 6 | 50,492 / 1,039 | 0.15 / 0.50 | 0.00809330 |
| Kimi main | 1 | Unknown / unknown | 3.00 / 15.00 | Unknown |
| Kimi Guardian | 6 | 50,499 / 1,078 | 0.15 / 0.50 | 0.00811385 |

Across **all nine invocations**, the canonical campaign retains **58 observations**:
34 paid attempts (33 with measured usage), 24 synthetic proposal/continuation
requests, and zero auxiliary attempts. The known subtotal is **USD 0.07110715**;
the complete total is unknown because Kimi's timed-out request returned no usage.
Observed token subtotals are 272,731 input and 4,932 output; these exclude that
unknown usage. Initial setup failures produced no inference observations; their
reports retain null cost measurements, not invented zero-token usage.

Generated naming was not requested or measured. Provisional first-message titles
are distinct from generated titles. Only the recognized Kimi naming route is
supported; other-main naming remains explicitly unsupported, with no hidden Kimi
fallback. No title-quality, pure provider-compute, SLA or broad reliability claim
is made by this small workload.

## Earlier failures and qualification checks

The [canonical campaign](evidence/desktop-first-pair-2026-09-29/campaign.json)
retains every invocation, including diagnostics, and counts each run once.
[Run annotations](evidence/desktop-first-pair-2026-09-29/run-annotations.json)
identify invocation conditions. Exports are not new accounting inputs. The
aggregate matrix reports `multiple_conditions` for both attempted candidates;
the final-source DeepSeek group is `mixed` because a setup failure preceded its
passing baseline. The table above describes the two correctly invoked live
baselines, not an erasure of those earlier outcomes.

1. Initial DeepSeek and Kimi attempts failed before inference because disposable
   HOME hid the saved Keychain login. A separate startup diagnostic and
   [read-only catalog probes](evidence/desktop-first-pair-2026-09-29/initial/credential-probe.json)
   isolated that cause. A failing public-evaluator regression preceded the narrow
   Keychains-directory link fix. The engine's scratch file-auth policy remained
   intact. [Initial source and artifacts](evidence/desktop-first-pair-2026-09-29/initial/freeze.json)
   are retained separately.
2. The revised configuration's first DeepSeek and Kimi invocations used a relative
   campaign path. Child drivers could not persist observations/results relative
   to their scratch workspaces. Separate diagnostics captured fixed error
   categories and then `FileNotFoundError` code locations. The observer checkpoints
   before forwarding inference, so no provider requests were sent by these failed
   invocations. Correcting the command to an absolute campaign path required no
   source, routing, workload or deadline change. Raw client output was discarded;
   [diagnostic tool provenance](evidence/desktop-first-pair-2026-09-29/diagnostic-tools.json)
   identifies the extra instrumentation.
3. One initial controlled GLM Flash case returned `client_incomplete` after a
   successful allow/execution. Its cause remains unresolved; the
   [failed report](evidence/desktop-first-pair-2026-09-29/initial/glm-flash-controlled-initial.json)
   and [unchanged passing repeat](evidence/desktop-first-pair-2026-09-29/initial/glm-flash-controlled-repeat.json)
   remain visible. These synthetic observations are not live model outcomes.
4. The first full Go race run exposed a test-fixture startup race: its fake desktop
   exited before the installed engine was ready. The unchanged test failed 3/5
   repeats. Waiting for an app-server initialization response made 5/5 repeats
   pass; the subsequent full race suite passed. Production lifecycle logic and
   inference deadlines were unchanged.

[Checks and coverage gaps](evidence/desktop-first-pair-2026-09-29/checks.json)
record the full Go race suite, `go vet`, format/syntax checks, the 16-test desktop
harness suite, 34 evaluator tests, 13 observer tests, and the remaining Python,
terminal, build and installation checks. Matching controlled
[DeepSeek](evidence/desktop-first-pair-2026-09-29/final/deepseek-controlled.json)
and [Kimi](evidence/desktop-first-pair-2026-09-29/final/kimi-controlled.json)
reports passed before the final live runs. Installed Electron UI, optional
standalone Codex CLI, and native Windows coverage were skipped where unavailable;
standalone desktop-picker skips are covered by its actual Go parent fixture.

## Retained artifacts and continuation

The private, git-ignored `.qualification/desktop-2026-09-29-v2/` directory retains
the final launcher, engine, source tree/archive/patch, controlled-provider reports
for all five candidates, raw sanitized campaign checkpoints and report exports.
The original `.qualification/desktop-2026-09-29/` remains intact. The combined
`.qualification/desktop-campaign-2026-09-29/` links each immutable run once for
report recovery. Keep these directories for the remaining candidate tickets.

From the repository root, recover the complete campaign without inference:

```sh
artifact="$PWD/.qualification/desktop-2026-09-29-v2"
python3 "$artifact/source/scripts/model_evaluation.py" \
  --desktop-report "$PWD/.qualification/desktop-campaign-2026-09-29" \
  --output /absolute/path/to/new-campaign-report.json
```

Subsequent candidates must use the retained source, launcher and engine, plus a
matching successful controlled report for that exact main. Use an absolute
`--campaign` directory and a new output filename; live preflight must refresh
availability, exact metadata and prices again. Changed source, engine, routing or
workload requires a new freeze and separate evidence. Additional investigations
use `--diagnostic`, preserving these comparable baselines.

An ordinary experimental launch with this tested combination is:

```sh
tofa launch codex-desktop --model deepseek-ai/DeepSeek-V4.1-Flash \
  --guardian-model zai-org/GLM-5.3-Flash --allow-unverified
```

Arrange any active-desktop transition before an ordinary launch. Existing
conversations retain their recorded main/provider; resume them by relaunching
with their original main. Follow the [desktop recovery instructions](../codex-desktop.md).
