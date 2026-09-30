# Desktop rc.6 qualification preparation — 2026-09-29

**Status: local automated checks and exact-source CI passed; maintainer UI run pending.**
This is a checkpoint for [#35](https://github.com/kreuzhofer/tofa-launcher/issues/35),
not a completed qualification or published release.

The [rc.5 preparation](desktop-rc5-preflight-2026-09-29.md) is retained. Its two
source-CI attempts exceeded the ten-minute native macOS job limit, so the job
budget is now twenty minutes. Client request deadlines and checks are unchanged.
That source change requires a fresh candidate; rc.5 assets were not replaced.

## Candidate and checks

- Source: `5f4793f0865192e3f30c0f612784da66483e2075`.
- Version: `v0.1.0-rc.6`, reserved locally; tag not pushed.
- macOS ARM64 SHA-256:
  `bb3a3e337fda0d3cbcedb4f7b0acdd4091622cccf0d80f7062e28a7815348530`.
- ChatGPT 26.917.71314 (10954), bundled engine 0.155.0-alpha.16.4,
  macOS 26.6.2 ARM64.
- Main `deepseek-ai/DeepSeek-V4.1-Flash`; Guardian `zai-org/GLM-5.3-Flash`.
- Capsule: `.qualification/desktop-rc6-2026-09-29/`.

The [evidence JSON](evidence/desktop-rc6-preflight-2026-09-29.json) records all 18
asset hashes, exact app/engine hashes, helper hashes and check results. Embedded
build information confirms Go 1.27.1, the rc.6 module version, source revision and
`vcs.modified=false`.

The full installed-engine/desktop race run passed 423 tests/subtests. Its two
optional standalone CLI inference tests were skipped. Its source code and test
files are byte-identical to rc.6: only workflow budget and preparation documents
changed. The fifteen earlier static/packaging/offline checks remain applicable;
rc.6's build, installed lifecycle and terminal checks also passed separately.
An initial build-test invocation lacked Go in PATH; the corrected explicit-toolchain
invocation passed all six tests. No live inference or ordinary-account installation
has been performed during this preparation.

[Exact-source CI](https://github.com/kreuzhofer/tofa-launcher/actions/runs/36608209151)
passed artifacts, Linux, macOS and Windows. Publication was correctly skipped
because the release tag has not been pushed. The earlier cancellations remain
recorded separately.

## Maintainer UI procedure

**September 30 update:** the agent has completed the real installer/uninstaller
checks below. Do not ask the maintainer to repeat the terminal wizard for those
steps. The remaining handoff is for visible UI observations; the agent handles
CLI lifecycle operations. The original wizard command is retained as historical
procedure, not a requirement to rerun completed installation checks.

Computer Use refused access to `com.openai.codex`; the visible real-account checks
therefore require the maintainer. Run from the repository root:

```sh
python3 .qualification/desktop-rc6-2026-09-29/observe.py run
```

The one-run wizard covers ordinary baseline; upgrade and streaming/tools/continued
conversation; actual automatic approval, cancellation and clean exit; ordinary
return and same-conversation relaunch; uninstall without purge; reinstall with
saved login; final preservation. It asks for fixed pass/fail observations only.
Use its synthetic workspace and prompts. Three owned launcher sessions have
15-minute outer limits; two short native-account replies are also requested.
There is no currency cap. Stop at the first failure and retain that attempt.

The recorder rejects inherited routing/install overrides and shell-selected
alternate `CODEX_HOME`. It supports default-home installations, with either no
managed activation or one standard final zsh PATH block. Uninstall removes that
block; reinstall restores it through the ordinary installer. Startup content and
activation metadata are compared against the original or the exact expected
installer result (one additional separating newline). Nonstandard activation
requires inspection before running; no unrelated settings are rewritten.

Settings and credential-file fingerprints stay in memory; only equality booleans
are reported. Native Keychain values are not read. Saved-login reuse is a visible
observation, not an independent vault comparison. SIGTERM and interruption stop
the owned wizard and launcher through bounded cleanup. Each run saves a separate
private `ui-*/report.json`; failures remain incomplete. Private launcher logs require
sanitization before publication. No credentials, private titles or conversation
content should be copied into evidence. No snapshots are restored to hide drift.

The helper passed six synthetic process checks, including shell-only home overrides,
termination cleanup and actual install/uninstall PATH recovery in a temporary home.
Standards and Spec re-review each have zero remaining findings. These checks do not
qualify the real UI.

## Remaining release gate

Keep #35 open until the UI report passes, final evidence
is reviewed, release notes/install instructions identify the qualified boundary,
and tag CI publishes and verifies the immutable candidate. Compare published bytes
against these hashes; any mismatch or further source change requires investigation
and a new candidate. The helper never publishes or pushes tags.

DeepSeek automatic naming is explicitly unsupported; verify the announced limitation
and usable provisional title. No Lightning route, live compaction or additional
auxiliary behavior is qualified. Windows desktop remains unqualified. GLM Flash and
Kimi main remain experimental. The five-model campaign is not repeated. Published
candidate real-computer acceptance remains the separate #23 task.

## First UI attempt and environment recovery

The 21:08:52–21:10:10 UTC attempt passed the ordinary native baseline, then stopped
at upgrade with `incompatible desktop bundle: expected CFBundleIdentifier=com.openai.codex`.
All five recorded settings/credential/workspace preservation comparisons passed.
No candidate desktop session or candidate inference began.

A read-only replay of upgrade's recorded-bundle checks reproduced the error:
69 bridge records referenced deleted temporary `Evaluation.app` bundles. The
headless evaluation harness shared the saved-login configuration directory but
left its disposable bridge records behind. The installed ChatGPT bundle remained
exactly qualified; its engine and app hashes still matched the rc.6 freeze.

The 69 orphaned records and matching executables were verified for path ownership,
private permissions and checksums, then moved to a private reversible quarantine.
The real ChatGPT bridge was retained byte-for-byte. The same recorded-bundle check
then passed. This is an evaluation-harness state leak, not evidence of an
incompatible installed app or provider failure. The harness now retires only its
own verified bridge after the launcher stops, including failed launches; conflicting
or preexisting records fail explicitly.

The original report and frozen rc.6 assets remain unchanged. Run the same manual
command again to create a separate attempt. Recovery does not count as a successful
upgrade or UI qualification. The harness correction is outside rc.6's frozen
installable artifacts; this retry continues to test the original candidate bytes.

Recovery validation: the focused cleanup and report-preservation regressions passed,
as did all 34 model-evaluation tests and the final offline suite (23 tests, 16
installed-engine opt-ins skipped). The separate opt-in engine suite passed 21 of
22 tests. One approval case stopped during client startup after 488.552 ms, before
engine setup or any provider request; its isolated integration-test rerun passed
in 30.956 seconds. Both results are retained, and the initial full suite is not
reported as all green. All these requests used synthetic loopback providers.
Standards and Spec re-review each have zero remaining findings.

## Agent-run installed lifecycle — September 30

The maintainer's second wizard attempt installed rc.6 successfully, then waited
for confirmation. The agent verified the installed checksum and stopped that
waiting recorder gracefully, retaining its incomplete report and successful
preservation comparisons. This hands executable lifecycle checks back to the
agent instead of requiring further maintainer terminal input.

The agent then ran the frozen **real `install.sh` and `uninstall.sh`** against the
ordinary installation: repeat install, uninstall without purge, and reinstall.
All commands exited zero. The exact rc.6 binary is installed again. Authenticated
catalog checks before and after the sequence confirmed saved-login reuse and
availability of the selected main and Guardian, without paid inference.

[Installed lifecycle evidence](evidence/desktop-rc6-installed-lifecycle-2026-09-30.json)
records passing configuration, credential-file, Electron-preference, workspace,
and existing-history preservation checks after each phase. History checks compare
existing conversation-log prefixes and stored thread identity/title/archive metadata;
ongoing conversation appends and SQLite activity-file changes are permitted.
An earlier bytewise history comparison stopped after successful repeat installation
and remains a distinct incomplete attempt. No history snapshots were restored.

Visible shared-history parity, live tools and continuation, automatic approval,
cancellation and same-conversation UI recovery still require observation. This
installed lifecycle result does not claim those UI checks or complete #35.

## Normal PATH setup correction — September 30

The earlier lifecycle used `--no-modify-path` when no managed activation existed.
That verified installation at its absolute location but left normal terminal
setup incomplete. The maintainer identified that `tofa` was not on PATH.

The agent stopped its owned test launch, ran the frozen installer with default
PATH modification enabled, and verified a new interactive login zsh starting with
only `/usr/bin:/bin:/usr/sbin:/sbin`. `command -v tofa` resolved the installed
binary and `tofa --version` returned rc.6. Installer-owned activation metadata
exists; configuration and credential-file preservation checks passed. Existing
terminal processes retain their previous environment until a new shell starts.

[PATH setup evidence](evidence/desktop-rc6-path-setup-2026-09-30.json) records this
separately from the earlier limited lifecycle result. The agent then restarted
the desktop test using `tofa` resolved through that ordinary shell PATH. Main
and Guardian startup announcements were observed; visible history remains a
human observation, not a claimed automation result.
