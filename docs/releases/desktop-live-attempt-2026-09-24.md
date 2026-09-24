# Shared-history live attempt — 2026-09-24

**Historical outcome: partial live success on candidate `fd22182`.**
Across the first attempt and its continuation, 29 human checks passed. The
captured shutdown failure and four final preservation checks were addressed in
the corrected candidate's [separate final run and reviewed qualification](desktop-shared-history-final-2026-09-24.md).
The sections below retain each attempt's original results and candidate identity.

The human walkthrough reached the first return to ordinary mode, then stopped
because the launcher exited with status 1. Sixteen human observations passed;
automatic title generation was not observed. The remaining seventeen human
observations were not collected. Do not interpret the end of the script as
completion of all eight stages.

This is follow-up evidence for [#50](https://github.com/kreuzhofer/nebius-tofa-cli/issues/50),
under #44/#34, and an input to
[prerelease tracking #35](https://github.com/kreuzhofer/nebius-tofa-cli/issues/35).
No release, expanded support claim or issue closure follows from this attempt.
The [earlier qualification record](desktop-shared-history-2026-09-24.md) and its
candidate evidence remain separate.

## Exact candidate and evidence

The [sanitized report](evidence/desktop-live-attempt-2026-09-24.json) is an unchanged
copy of the private wizard's final report, SHA-256
`803a06d760ada17ce95d3d49d5324cdea153de8d2892b07f5349d464e7648408`.
It contains outcome enums, boolean checks, counts, timestamps and build identities;
no credential, conversation body, account ID or title text is included.

| Component | Observed identity |
| --- | --- |
| Production commit | `fd221828e61dcc231973b6e3846f7dc1513c8395` |
| Candidate | `v0.0.0-qualification.50.fd22182`, clean git archive, Go 1.27.1, CGO disabled, trimpath and version ldflag |
| Candidate SHA-256 | `46362c39507f0c9e23c2aef9718f6fd192d5a53d3c72aeae9c1c41f10b07f676` |
| Desktop | ChatGPT 26.917.71314 build 10954, `com.openai.codex`, Codex mode |
| Bundled engine | `codex-cli 0.155.0-alpha.16.4` |
| Platform | macOS 26.6.2 ARM64; recorded build 25G83 |
| Model | `moonshotai/Kimi-K3`, `--allow-unverified` |
| Observation interval | 14:03–14:13 UTC, 2026-09-24 |

Initial and pre-launch binary/app/platform identity checks passed. App component
hashes are in the report. Final identity checks were not reached.
The [login-policy correction](../research/desktop-login-policy-diagnosis.md)
enabled this attempt; the earlier candidate's refusal is separate evidence.

## Human observations

| Stage | Recorded result |
| --- | --- |
| 1. Ordinary baseline | Signed-in account usable; native marker reply, unique history entry/title/workspace and native picker inspection passed. |
| 2. First tofa launch | Usable UI, account/onboarding continuity, list parity, native picker parity and continuation of the original native conversation passed. |
| 3. Real Kimi conversation | Selected Kimi, visible incremental streaming, successful shell tool marker, same-conversation follow-up and complete messages/tool result passed. |
| 3. Automatic title | `not_observed`. The operator separately reported that the title remained the first user message; no generated title or explicit title error was observed. |
| 4. Picker limitation | Explicit failure after selecting native GPT/Astra in the Kimi conversation, with earlier history still readable, passed. |
| 5. Return to ordinary | Stopped at launcher-exit validation, before ordinary-mode UI observations. |
| 6–8. Ordinary send, second tofa launch, final cleanup | Not reached. |

Streaming is human-observed, not a captured wire-event count. The title fallback
observation does not establish title generation or its explicit-error path, and
does not yet establish stable titles across modes. Automatic titles retain their
documented limitation. Successful real inference in this first launch does not
qualify the complete history/recovery workflow.

## Automated checks and blocker

Ordinary-account native discovery passed at baseline and immediately before the
first tofa launch: nine descriptors matched a fresh account cache and differed
from the bundled catalog. The launch-owned catalog preserved all nine full native
descriptors and added exactly one Kimi descriptor. The native catalog did not
change between these two exports. Saved Token Factory model availability passed.

Production startup was recorded at 14:05:01 UTC. The launcher exited with code 1
at 14:13:33 UTC. At 14:13:42 UTC, its PID was absent and its launch runtime had
been removed, but the nonzero exit caused the cleanup gate to fail and the wizard
to stop. The launcher PID was absent and its runtime directory was removed; the
overall cleanup gate failed because the exit was nonzero. These checks do not
independently establish that every owned desktop/engine process or the adapter
listener was gone. Raw terminal diagnostics were deliberately not retained by
the wizard, and the operator subsequently reported that the launcher Terminal
had closed. The exit cause cannot be inferred from this report alone.

The candidate's prior full race suite passed in 254.612 seconds with installed
desktop/engine opt-ins enabled, and `go vet ./...` passed. Two installed CLI
opt-ins skipped because `TOFA_TEST_CODEX` was unset, as documented in the
[login-policy diagnosis](../research/desktop-login-policy-diagnosis.md).
Those offline results do not override this newly observed live exit failure.

Resolve the exit failure before claiming qualification. Remaining observations
include ordinary-mode history and both logo-loop checks, explicit inactive send
failure, second-launch same-conversation Kimi recovery, native continuation and
picker parity after relaunch, and final ordinary-mode preservation. Retain the
two qualification conversations so those checks can use the existing history.

## Follow-up launch/quit diagnostic — 14:24 UTC

The operator ran a separate launch-and-quit check using the unchanged candidate.
The [sanitized diagnostic report](evidence/desktop-quit-diagnostic-2026-09-24.json)
has SHA-256 `f145ce6063757f49e826c75eff14594f6f19080e911baef5f56f784f7c66d21b`.
Candidate/app/platform identity and native-account freshness checks passed. The
operator recorded a usable desktop and quit method `cmd_q`; the launcher exited
with code 0 and the diagnostic helper reaped it. No launcher error category was
recorded. The helper retained only recognized error categories, never raw output.

The earlier exit-1 failure **did not reproduce** in this short check. This does
not establish a fix or explain the earlier cause: the short diagnostic did not
repeat the inference and picker interactions preceding the original failure.
It also did not collect ordinary-mode history or relaunch-recovery observations.
Those remain pending, with the two existing conversations retained for stages
5–8. A continuation is separate evidence and must record this intervening
launch/quit session; it must not rewrite the original partial attempt as a
successful uninterrupted walkthrough.

## Continuation — 14:33–14:37 UTC

The [unchanged continuation report](evidence/desktop-continuation-2026-09-24.json)
has SHA-256 `2e4483fe2ea50fc95d744cad84138a664b4a626526d911841e8ea7a5c73e7360`.
It pins the earlier two reports and the same `fd22182` production candidate.
Thirteen further human observations passed:

- Ordinary account continuity, list/title/workspace parity and complete messages
  and tool output; both saved-engine and provider-history logo loops were absent.
- Ordinary-mode inference failed explicitly while history remained readable.
- Tofa relaunch became usable with the same account; the existing Kimi
  conversation recovered its earlier marker and retained its history.
- The original native conversation continued successfully and native picker
  choices remained available.

Fresh native discovery and launch-owned catalog checks again passed with nine
native descriptors preserved and exactly one Kimi entry added. Startup was
recorded at 14:36:05 UTC. At final cleanup the operator used Cmd-Q, confirmed
separately in the conversation. The launcher again exited with status 1, this
time recording `engine_exited_before_app` at 14:37:42 UTC. That category maps
exactly to `owned desktop app-server exited; launch cancelled`. The launcher PID
was absent and its runtime directory removed at the subsequent check; no broader
process-cleanup claim is made.

Across the first attempt and continuation, 29 human observations passed and
automatic title generation remains `not_observed`. Four final ordinary-mode
observations—account, history, workspace and settings preservation—were not
collected because the cleanup gate stopped the continuation. Final identity
checks were also not reached. The unchanged reports remain incomplete.

The [shutdown diagnosis](../research/desktop-shutdown-diagnosis.md) reproduces
the captured error with an engine-first clean shutdown and records the scoped
correction. It requires live confirmation before this combination can be claimed
qualified. Earlier successful checks are retained as evidence for their exact
candidate rather than rewritten as observations of a new binary.
