# Desktop login-policy refusal — 2026-09-24

The first human-run #50 walkthrough stopped before Codex started. Candidate
`v0.0.0-qualification.50.1e49a11` (SHA-256
`d069d4608e62103df4d0d976f755442abdc56c79e77e7a00490581daa093065c`)
exited with status 1 at 13:46:32 UTC, reporting
`managed allowedLoginMethods requirement is not qualified for desktop routing`.
Its identity and native-catalog preflights passed, but there was no production
startup announcement. The maintainer confirmed that the app did not start.
This is a launcher refusal, not evidence of a desktop UI failure.

## Reproduction and cause

An authorized read-only `configRequirements/read` query through the installed
alpha.16.4 engine returned `allowedLoginMethods: ["api", "chatgpt"]`.
The query used production's login-shell home resolution, scrubbed environment
and the walkthrough workspace. No credential values or other policy contents
were exported. No login policy was edited.

The production `App.Run` regression reproduced the exact refusal in 0.86 seconds
using that response at the existing executable fixture boundary. With the
otherwise identical fixture's absent requirements, the normal launch succeeds.
The ranked hypotheses were: a non-null check rejects an unrestricted set; the
two values have more restrictive semantics; or another restriction is also
blocking launch.

The first hypothesis was confirmed. The guard treated every non-null login-method
field as an unsupported restriction. Codex's
[pinned auth policy](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/config/src/auth_policy.rs)
computes the effective subset of the two methods. Its
[configuration API mapping](https://github.com/openai/codex/blob/4607249e430dac1c961df4dc615beae88e33cec8/codex-rs/app-server/src/request_processors/config_processor.rs)
includes both methods when other requirements exist, even without a login
restriction. These source facts explain the observed installed-engine response;
they do not qualify additional client versions.

## Correction and checks

Tofa now accepts exactly the unrestricted two-method set, in either order.
Absent/null values keep their existing behavior. Restricted, empty, malformed,
duplicate and unknown-method values still fail closed, as do the other guarded
managed routing requirements. No auth setting, credential, compatibility pin or
desktop routing mechanism changed.

`TestDesktopManagedLoginMethods` exercises launch and refusal through `App.Run`.
Its nine cases include both-method success and a simultaneous provider restriction
that must still prevent startup. Together with the existing managed-routing test,
the targeted run passed in 9.785 seconds. The original success case was run red
before the production change and green afterward.

The corrected production preflight also passed against the ordinary signed-in
profile in 1.37 seconds using a private temporary diagnostic with `ownerPID=0`.
That path performs public configuration reads and returns before any write or
desktop launch. It therefore rules out another restriction at this preflight
boundary, without claiming that the later launch or UI checks have passed. The
diagnostic was removed from the test package and retained only in private scratch.

`go vet ./...` and the full `go test -race ./... -count=1` suite passed
(`internal/tofa`: 254.612 seconds), with `TOFA_TEST_DESKTOP_ENGINE` and
`TOFA_TEST_DESKTOP_APP` pointing to the installed alpha.16.4 engine and app.
The separate installed CLI tests `TestInstalledCodexToolAndContinuationThroughAdapter`
and `TestInstalledCodexApprovalReviewThroughAdapter` skipped because
`TOFA_TEST_CODEX` was unset. No desktop opt-in skipped. Standards and Spec reviews
found no actionable findings in this scoped correction; #50's remaining live
acceptance checks are still open.

## Retry boundary

The failed wizard attempt and its pinned candidate remain intact in private local
scratch. A retry must use a newly built, separately pinned candidate; rerunning the
old launch command would reproduce the refusal. The user-approved eight-stage
walkthrough order is unchanged. This correction and the read-only preflight do
not complete [#50](https://github.com/kreuzhofer/nebius-tofa-cli/issues/50): real
desktop startup, history, streaming/tools, ordinary-mode failures and relaunch
recovery still require the human observations.
