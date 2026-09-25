# Shared desktop history: reviewed qualification — 2026-09-24

**Outcome: the scoped experimental shared-history workflow passed.** The final
candidate passed live conversation recovery, native continuation, clean shutdown
after inference, and ordinary-mode account/history/workspace/settings preservation.
The combined evidence covers 33 distinct workflow observations. Automatic title
generation was not observed and remains a documented limitation.

This is the qualification result for
[#50](https://github.com/kreuzhofer/nebius-tofa-cli/issues/50), under #44/#34, and
an input to [desktop prerelease tracking #35](https://github.com/kreuzhofer/nebius-tofa-cli/issues/35).
It does not publish a release, remove `--allow-unverified`, expand compatibility,
or close either parent. Broader release and real installed-account lifecycle
qualification remain with #35.

## Exact final candidate

| Component | Identity |
| --- | --- |
| Production source | `b7720045227b3ee0f5c577fac6efbc2a1314eead` |
| Version | `v0.0.0-qualification.50.b772004` |
| macOS ARM64 binary SHA-256 | `713bb2444ea985ad8cd0215c6904332b055c228b2a8384deaa0cb01e1fd4a1d1` |
| Desktop | ChatGPT 26.917.71314, build 10954, `com.openai.codex`, Codex mode |
| Bundled engine | `codex-cli 0.155.0-alpha.16.4` |
| Platform | macOS 26.6.2, recorded build 25G83, ARM64, `/bin/zsh` login shell |
| Token Factory model | `moonshotai/Kimi-K3`, mandatory `--allow-unverified` |
| Construction | Clean git archive; Go 1.27.1; CGO disabled; trimpath and version ldflag |

The [final run](evidence/desktop-final-run-2026-09-24.json) pins all app component
hashes and records successful identity checks before, during and after the run.
Its SHA-256 is `753ac9d0acababf9c4d0678f4dbe50db1bd949b47f3791faa03253ca2f2fe91a`.
The [six-platform build manifest](evidence/desktop-final-build-2026-09-24.json)
reproduces the exact live macOS ARM64 binary. Cross-builds do not establish native
Linux, Windows or Intel desktop compatibility. Existing refusal gates remain.

## Evidence attribution

This was an incremental qualification with an intervening diagnostic, not one
uninterrupted all-pass run. Earlier reports and their failures remain unchanged.

| Evidence | Candidate | What it establishes |
| --- | --- | --- |
| [First live attempt](evidence/desktop-live-attempt-2026-09-24.json), 14:03–14:13 UTC | `fd22182` | 16 human passes: ordinary baseline, account/list/picker continuity, native continuation, real Kimi streaming/tool/follow-up, and explicit picker refusal with readable history. Exit status 1 stopped this attempt. |
| [Short quit diagnostic](evidence/desktop-quit-diagnostic-2026-09-24.json), 14:24 UTC | `fd22182` | Usable desktop and clean Cmd-Q without the earlier inference/picker sequence. Did not explain the first failure. |
| [Continuation](evidence/desktop-continuation-2026-09-24.json), 14:33–14:37 UTC | `fd22182` | 13 further human passes: ordinary history/account continuity, stable titles/workspaces and full tool output, absence of both logo loops, explicit inactive send refusal, same-conversation Kimi recovery, and native continuation/picker parity. Cmd-Q captured an engine-first shutdown refusal. |
| [Corrected candidate run](evidence/desktop-final-run-2026-09-24.json), 15:06–15:08 UTC | `b772004` | Six recovery/native/UI observations repeated successfully, followed by clean launcher exit and all four final ordinary-mode preservation checks. Ten human passes, no missing observations or automated checks within this run's scope. |

The intervening production change is limited to the
[bounded shutdown correction](../research/desktop-shutdown-diagnosis.md).
The earlier streaming, tool-execution, picker-refusal and ordinary inactive-send
observations are attributed to `fd22182`; they were not repeated and relabelled
as observations of `b772004`. The corrected candidate repeated live Kimi and
native continuation before quitting, exercising the activity pattern preceding
the captured failure, and preserved the same conversations afterward. Regression
coverage on the final source verifies the unchanged routing and history behavior.

The final helper report intentionally retains `end_to_end_qualification_claimed:
false` and the historical reference `earlier_exit_failure_resolved: false`.
The helper does not make review decisions or rewrite earlier failures. This
document supplies the reviewed disposition: the captured shutdown defect has a
regression, a scoped correction, and a successful live confirmation. The first
unclassified exit cannot be assigned a cause retrospectively.

## Acceptance and limits

- Human observations establish the ordinary → tofa → ordinary → tofa workflow,
  list and message/tool preservation, stable titles/workspaces, both logo-loop
  checks, account/onboarding continuity, explicit ordinary-mode send failure and
  picker limitation, and same-conversation recovery.
- Authorized ordinary-account exports matched fresh native caches. Each inspected
  launch preserved nine full native descriptors and added exactly one Kimi entry.
  Native picker choices and native continuation also passed human checks. No
  account or entitlement change was induced; those variations retain synthetic
  regression coverage rather than new live claims.
- Real Token Factory streaming, shell-tool execution and follow-up passed human
  observation on the production route. Streaming has no exported wire-event
  count. Credential contents, account/project IDs and conversation text were not
  copied into the reports or fixtures.
- The original Kimi title stayed at the first-message fallback. This does not
  establish automatic title generation or its explicit error path. Title
  preservation passed; automatic titles, automatic review, auxiliary models and
  compaction retain the existing documented limitations.
- The corrected run's launcher exited 0; its PID was absent and its runtime
  directory removed. These checks do not independently enumerate every descendant
  process or listener. Owned-process/listener cleanup has separate regression
  coverage. The final four human preservation checks all passed.

## Validation and lifecycle documentation

The final source passed `go test -race ./... -count=1 -timeout=6m` in 247.720 seconds
for `internal/tofa`, with installed desktop and engine opt-ins enabled, and
`go vet ./...`. The two installed CLI opt-ins skipped because `TOFA_TEST_CODEX`
was unset. No desktop opt-in skipped. The
[diagnosis](../research/desktop-shutdown-diagnosis.md) retains the earlier failed
suite attempts, fixture corrections, red/green results and review outcomes.

The final clean source also built all six distribution artifacts. The four native
candidate lifecycle tests passed (4.662 seconds), and both installer fixture tests
passed (0.866 seconds). These use disposable homes and synthetic credentials; they do not
replace #35's real installed-account upgrade/uninstall qualification.

[User instructions](../codex-desktop.md) cover shared history, launch-only
inference, unavailable-provider recovery, exact compatibility and retained
non-secret bridge/provider state after uninstall. The
[initial production attempt](desktop-shared-history-2026-09-24.md),
[live-attempt chronology](desktop-live-attempt-2026-09-24.md) and immutable
prototype records remain provenance. One-off wizard tooling stays in ignored
scratch; no harness is shipped or installed.
