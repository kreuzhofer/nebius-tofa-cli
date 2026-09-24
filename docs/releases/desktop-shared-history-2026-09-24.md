# Shared desktop history qualification — 2026-09-24

**Outcome: incomplete; this combination is not qualified end to end.**
This record describes the initial candidate below. Subsequent
[native-catalog recovery](../research/desktop-native-catalog-diagnosis.md#authorized-ordinary-profile-recovery--1322-utc)
passed; the first human-run launch then exposed a
[login-policy validation bug](../research/desktop-login-policy-diagnosis.md)
before the app started. Those later checks do not replace this candidate's evidence.
The [corrected candidate's human walkthrough](desktop-live-attempt-2026-09-24.md)
subsequently passed the first live inference/history checks, then stopped on a
nonzero launcher exit before the ordinary-mode return checks.

This is the production-path qualification record for
[#50](https://github.com/kreuzhofer/nebius-tofa-cli/issues/50), under
[#44](https://github.com/kreuzhofer/nebius-tofa-cli/issues/44) and
[#34](https://github.com/kreuzhofer/nebius-tofa-cli/issues/34).
The integrated implementation passes the checks below, but the ordinary account's
static custom catalog does not meet the existing freshness contract. UI and live
inference requirements remain open. No release was published, compatibility gate
widened, or parent issue closed.

This result is an input to
[desktop prerelease tracking #35](https://github.com/kreuzhofer/nebius-tofa-cli/issues/35),
not its immutable release-candidate qualification. The rc.2
[CLI lifecycle runner](qualification.md) cannot substitute for desktop acceptance.
Dependencies #48 and #49 were closed when this attempt began.

## Exact production source and candidate

| Component | Tested identity |
| --- | --- |
| Production source | `933447577e008f4a94929c8816d5c5ff5e1d4e31` (#49, including #45–#48) |
| Local candidate version | `v0.0.0-qualification.50.9334475`; no release/tag created |
| Candidate construction | Clean `git archive` of that commit; `sh scripts/build.sh v0.0.0-qualification.50.9334475` |
| macOS ARM64 candidate SHA-256 | `76c13df565c356ea9777ae2fc1bfeb455b46b04395089a51a327909aa675c86c` |
| Desktop | ChatGPT `26.917.71314` (`10954`), `com.openai.codex`, Codex mode |
| Bundled engine | `codex-cli 0.155.0-alpha.16.4` |
| Platform | macOS `26.6.2` (`25G83`), ARM64; `/bin/zsh` login shell |
| Build toolchain | `go1.27.1 darwin/arm64` |
| Token Factory model | `moonshotai/Kimi-K3`; mandatory `--allow-unverified` |
| Connection | Launch-owned authenticated loopback Responses adapter; real Token Factory credentials remain in the launcher |

The [sanitized evidence](evidence/desktop-shared-history-2026-09-24.json) contains
the candidate/script hashes and fresh installed app, engine, archive and framework
hashes. The candidate was built in a disposable checkout; the user's installed
launcher was not replaced. Lifecycle tests installed the exact local candidate in
temporary homes. Go integration tests exercised the same production source.
The subsequent #50 commit changes documentation/evidence only.

Fresh identity checks agree with the existing production gate, so it stays pinned.
Neither the older alpha.9.2 foundation checks nor the alpha.16.3 prototype is used
to qualify alpha.16.4. Other application, engine, OS, architecture, login-shell and
model combinations retain their existing actionable refusal. A binary match alone
does not establish ordinary-account/UI support.

## Evidence boundaries and acceptance status

| Requirement | Fresh evidence | Remaining gap |
| --- | --- | --- |
| One history; ordinary → tofa → ordinary → tofa | Production launcher plus actual alpha.16.4 engine, synthetic inference and disposable profiles | Human/supported-UI observation of the complete sequence |
| List parity, complete messages/tool results, stable titles/workspaces | Public engine API regressions verify recorded identities, paginated ordered history and command results | Visible lists and expanded tool results in both modes; no duplicate or hidden conversations |
| Both demonstrated logo loops absent | Durable bridge and inactive-provider regressions pass | UI cold-open of Token Factory history and app restart through its saved bridge reference |
| Real account/onboarding continuity | Authorized engine `login status` confirms ChatGPT login in the ordinary login-shell-resolved home | Observe the same signed-in account without repeated onboarding across launch modes |
| Effective native catalog freshness | Authorized ordinary-engine export succeeds; static catalog/cache mismatch recorded below | Actual launch passing freshness, picker/full-descriptor parity, and refresh on relaunch |
| Native continuation during a tofa launch | Synthetic native-provider continuation passes | Real native conversation continued while the production tofa launch is active |
| Ordinary-mode send failure; picker limitation; same-session recovery | Actual engine/production adapter regressions pass with synthetic replies | UI error and recovery of the same conversation after a fresh Kimi launch |
| Token Factory streaming, tool use, continuation | Saved login successfully lists Kimi-K3 in the live project catalog | Real inference through this combined desktop production path; catalog availability is not inference evidence |
| Lifecycle and failure recovery | Offline production regressions and candidate lifecycle checks below | Ordinary-account UI preservation through real install/upgrade/removal remains part of #35 |

There are **no new human UI observations** in this record. The available Computer
Use skill requires `node_repl`, which was not available in this session; no private
renderer/debugging interface was substituted. Installed-app ownership/startup
checks use synthetic profiles and establish only their stated boundaries.

## Authorized live preflight and blocker

The maintainer authorized use of existing desktop/Token Factory logins, inference
requests and quit/relaunch cycles. No credential was copied into a fixture or
evidence, and no login/logout or ordinary configuration edit was performed.

The candidate's `models` command successfully accessed the saved Token Factory
project and found Kimi-K3. Separately, the installed engine's public `login status`,
`config/read`, and `debug models` boundaries were checked in the ordinary engine
home, resolved using the same desktop-specific interactive login-shell query as
production. Account IDs, project IDs, private paths, credential values, raw native
diagnostics and catalog contents are omitted.

The account is signed in using ChatGPT and has a configured `model_catalog_json`.
Its export contains 13 descriptors and differs from the bundled export. The cache
is recent, but contains seven descriptors, lacks the account `identity`, and has
`client_version: 0.154.0`. It does not match the export or the production-required
`0.155.0` cache version. This is insufficient evidence of live-account discovery.

This **preflight observation**, compared with the existing production guard, means
the account does not meet the launch contract. It is not a captured production
launch error: no ordinary-profile tofa launch was attempted. The documented static
custom-catalog refusal is retained; the guard was not relaxed, and user settings
were not removed to force a pass. An authorized operator must resolve the ordinary
custom-catalog setup before retrying the live walkthrough. Simply refreshing the
cache is insufficient if the static catalog still bypasses native discovery.
Supporting that setup requires separate qualification. `launch codex` remains the
CLI alternative.

Because this prerequisite failed, no desktop relaunch or paid inference was
performed in this attempt. Live streaming, tool execution, continuation and native
account functionality are **not run**, not passes. Historical isolated live
results and synthetic responses do not satisfy them.

## Checks run

All offline credentials and inference are synthetic. Native ownership diagnostics
run separately from launcher tests because their deliberately running ordinary
instances would correctly trigger incumbent refusal.

| Check | Result |
| --- | --- |
| `go vet ./...` | Passed |
| Both desktop opt-ins + `go test -race ./... -count=1` | Passed; `internal/tofa` 230.557 s; installed desktop startup check passed |
| `TOFA_TEST_DESKTOP_ENGINE=… go test ./internal/tofa -run '^TestDesktop' -count=1` | Passed: 40 top-level tests; installed-app check skipped here and enabled in final suite |
| `TOFA_TEST_DESKTOP_ENGINE=… python3 scripts/desktop_history_test.py -v` | Passed: 3 tests, 37.569 s; actual engine, synthetic state/inference |
| `TOFA_TEST_DESKTOP_APP=… python3 scripts/desktop_ownership_test.py -v` | Passed: 5 tests, 7.950 s; pinned hashes checked before/after, synthetic profiles |
| `python3 scripts/lifecycle_test.py <candidate-dist> v0.0.0-qualification.50.9334475 -v` | Passed: 4 tests |
| `python3 scripts/install_test.py` | Passed: 2 tests |
| `python3 scripts/terminal_test.py <candidate>` | Passed: 2 tests |
| `python3 scripts/live_compat_test.py -v` | Passed: 19 offline harness tests; no live inference |
| `python3 scripts/qualify_macos_test.py -v` | Passed: 18 offline runner tests; no ordinary-account lifecycle |
| `python3 scripts/release_test.py -v` | Passed: 6 tests; no publication |
| `python3 scripts/build_test.py -v` | Passed: 6 distribution tests |
| `sh scripts/build.sh v0.0.0-qualification.50.9334475` | Passed: Darwin/Linux/Windows × ARM64/AMD64; cross-compilation only |

Full results and unchanged before/after app hashes are recorded in the adjacent
evidence file. Two separate installed Codex CLI tests were skipped because
`TOFA_TEST_CODEX` was not set: `TestInstalledCodexToolAndContinuationThroughAdapter`
and `TestInstalledCodexApprovalReviewThroughAdapter`. All desktop opt-ins ran in
the final suite. No native Linux/Windows execution or desktop compatibility is
claimed. The live macOS/Windows prerelease qualification runners were not run;
their offline tests do not establish real release acceptance. No newly written
behavioral test was needed for this documentation/evidence-only change.

Independent Standards and Spec reviews found no standards violations or fixable
documentation defects. The Spec review retained three blocked acceptance groups:
UI round-trip/recovery, live-account/catalog/native continuation, and real Token
Factory streaming/tool/continuation. Passing offline checks does not close them.

## Completing the live qualification

Use the [launch and recovery instructions](../codex-desktop.md) after resolving
the ordinary account's catalog prerequisite. Retain this incomplete result and
record the next attempt separately with its exact candidate and app/engine hashes;
an auto-update invalidates the pinned combination. A human observer or supported
UI tool must supply UI evidence. Do not export private conversation bodies,
credentials, account/project identifiers or unredacted screenshots.

1. In ordinary mode, create a harmless native conversation in a disposable
   workspace. Record a sanitized title and a fixed marker such as
   `tofa50-native-ok`. Note the account/onboarding state, visible conversation
   list and native picker choices. Use the engine's public model export for
   full-descriptor comparison; the picker is a lossy view.
2. Quit the desktop, then launch the exact candidate with
   `launch codex-desktop --model moonshotai/Kimi-K3 --allow-unverified`.
   Record successful ownership/startup separately from UI readiness. Confirm the
   same account and list, and continue the existing native conversation. Record
   effective native catalog/cache freshness at launch and again after relaunch;
   do not infer entitlement changes from a synthetic fixture.
3. Start a new Kimi conversation. Ask it to execute `printf tofa50-live-ok`,
   show the returned marker and provide a longer explanation. Observe incremental
   text, the real successful tool call and its displayed output. Send a second
   user turn asking for the prior marker without rerunning the command. Record
   success/failure, whether actual streaming was visible, and title behavior.
   A complete response alone does not establish streaming.
4. Quit the app and wait for the launcher to exit. Open ordinarily, verify list
   parity and reopen both conversations. Check the full messages/tool results,
   titles and workspaces. Observe both history hydration and restart via the
   saved engine reference without the two demonstrated logo loops. Sending in
   the Kimi conversation must fail explicitly while leaving history readable.
5. Exercise the picker limitation without claiming provider migration. Select
   GPT/Astra in the Token Factory conversation and record the explicit failure.
   Quit and relaunch through tofa, reopen that same conversation, return to
   Kimi-K3 and demonstrate successful continuation with the prior marker intact.
   Verify ordinary native conversations still use their native provider.
6. Record cancellation and cleanup, then return to ordinary mode. Confirm account,
   history and unrelated workspace/settings preservation. Record failures and
   reruns separately. For real installed lifecycle acceptance, follow #35 using
   its immutable candidate and the documented retained-state removal contract.

Automatic titles may fail; read-only title lookups with tools remain unqualified.
Native auxiliary models such as `codex-auto-review`, compaction, web search and
Chat/Work/voice remain outside the Token Factory support claim. The existing
narrow title/review adaptations and experimental gate remain unchanged.

## Immutable provenance

The throwaway prototype is not shipped. Its
[final commit](https://github.com/kreuzhofer/nebius-tofa-cli/commit/5b1d4cd4306daacf4d4483f11d1f33704cd9bc7d),
[human history walkthrough](https://github.com/kreuzhofer/nebius-tofa-cli/blob/5b1d4cd4306daacf4d4483f11d1f33704cd9bc7d/internal/tofa/prototype_shared_history/live-history-evidence.json),
[durable-reference experiment](https://github.com/kreuzhofer/nebius-tofa-cli/blob/5b1d4cd4306daacf4d4483f11d1f33704cd9bc7d/internal/tofa/prototype_shared_history/durable-reference-evidence.json)
and [final #34 result](https://github.com/kreuzhofer/nebius-tofa-cli/issues/34#issuecomment-5808930101)
remain historical evidence with their original scope. The
[alpha.16.4 production ownership evidence](../research/evidence/desktop-profile-ownership-2026-09-24.json)
also remains unchanged; its then-open #48/#49 notes describe that earlier attempt.
